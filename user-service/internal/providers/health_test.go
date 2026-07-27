package providers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/mattn/go-sqlite3"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commonresources "github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/common/runtime/workerpool"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	"github.com/aegiscore/user-service/internal/router"
)

func TestPostgresHealthChecker(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:health_checker?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checker := postgresHealthChecker{name: "postgres.primary_db", db: db, timeout: commonresources.DefaultPostgresPingTimeout()}
	require.Equal(t, "postgres.primary_db", checker.Name())
	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)

	_ = db.Close()
	result = checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)
	require.NotEmpty(t, result.Message)
}

func TestRedisHealthChecker(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	checker := redisHealthChecker{name: "redis.cache_redis", client: client, timeout: commonresources.DefaultRedisTimeout}
	require.Equal(t, "redis.cache_redis", checker.Name())
	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)

	redisServer.Close()
	result = checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)
	require.NotEmpty(t, result.Message)
}

func TestCasbinPolicyHealthChecker(t *testing.T) {
	checker := casbinPolicyHealthChecker{engine: stubLastError{}}
	require.Equal(t, "rbac.casbin_policy", checker.Name())
	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)

	checker = casbinPolicyHealthChecker{engine: stubLastError{err: errors.New("load failed")}}
	result = checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)
	require.NotEmpty(t, result.Message)
}

func TestCasbinPolicyHealthCheckerRecoversAfterReloadSuccess(t *testing.T) {
	engine := &stubLastError{err: errors.New("initial load failed")}
	checker := casbinPolicyHealthChecker{engine: engine}

	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)

	engine.err = nil
	result = checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)
}

func TestWatcherHealthChecker(t *testing.T) {
	checker := watcherHealthChecker{watcher: stubWatcherStatus{running: true}}
	require.Equal(t, "rbac.policy_watcher", checker.Name())
	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)

	cases := []struct {
		name    string
		watcher stubWatcherStatus
	}{
		{name: "stopped", watcher: stubWatcherStatus{}},
		{name: "last error", watcher: stubWatcherStatus{running: true, err: errors.New("subscribe failed")}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := watcherHealthChecker{watcher: tt.watcher}.Check(context.Background())
			require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)
			require.NotEmpty(t, result.Message)
		})
	}
}

func TestProvideHealthChecksUsesResourcePingTimeouts(t *testing.T) {
	settings := serviceconfig.ResourceSettings{Redis: commonresources.RedisConfigs{
		"cache_redis": {Addr: "127.0.0.1:6379", Timeout: 2 * time.Second},
	}}
	checks := ProvideHealthChecks(HealthCheckParams{Resources: settings})

	require.Len(t, checks.Readiness, 4)
	postgres, ok := checks.Readiness[0].(postgresHealthChecker)
	require.True(t, ok)
	require.Equal(t, commonresources.DefaultPostgresPingTimeout(), postgres.timeout)
	redis, ok := checks.Readiness[1].(redisHealthChecker)
	require.True(t, ok)
	require.Equal(t, 2*time.Second, redis.timeout)
}

func TestProvideHealthChecksAppliesDefaultRedisPingTimeout(t *testing.T) {
	settings := serviceconfig.ResourceSettings{Redis: commonresources.RedisConfigs{
		"cache_redis": {Addr: "127.0.0.1:6379"},
	}}
	checks := ProvideHealthChecks(HealthCheckParams{Resources: settings})

	redis := checks.Readiness[1].(redisHealthChecker)
	require.Equal(t, commonresources.DefaultRedisTimeout, redis.timeout)
}

func TestRegisterRuntimeDependencyMetricsRegistersCollectors(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:runtime_metrics?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	runtimeCfg := config.Config{
		App: config.AppConfig{Name: "aegiscore-user-service-test", Environment: "test"},
		Observability: config.ObservabilityConfig{
			Metrics: config.MetricsConfig{Enabled: true, Path: "/metrics"},
		},
	}
	settings := serviceconfig.ResourceSettings{
		Redis: commonresources.RedisConfigs{
			"cache_redis": {Timeout: time.Second},
		},
	}
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      runtimeCfg.Observability.Metrics,
		ServiceName: runtimeCfg.App.Name,
		Environment: runtimeCfg.App.Environment,
	})
	require.NoError(t, err)

	err = RegisterRuntimeDependencyMetrics(RuntimeDependencyMetricsParams{
		Resources:        settings,
		Metrics:          provider,
		PrimaryDB:        db,
		CacheRedis:       client,
		SessionPurgePool: fakePurgeTaskPool{stats: workerpool.Stats{Name: "auth.redis.session_purge", Workers: 4, Submitted: 3}},
		PolicyWatcher:    stubWatcherStatus{running: true},
		AuthTokenCache:   fakeLocalcacheStatsSource{name: "auth_token_version", stats: localcache.Stats{Hit: 3, Capacity: 1000}},
		RBACRolesCache:   fakeLocalcacheStatsSource{name: "rbac_user_roles", stats: localcache.Stats{Miss: 2, Capacity: 2000}},
	})
	require.NoError(t, err)

	body := gatherProviderText(t, provider)
	for _, want := range []string{
		`aegiscore_postgres_pool_open_connections{environment="test",resource="primary_db",service="aegiscore-user-service-test"}`,
		`aegiscore_redis_up{environment="test",resource="cache_redis",service="aegiscore-user-service-test"} 1`,
		`aegiscore_workerpool_tasks_total{environment="test",event="submitted",pool="auth_session_purge_pool",service="aegiscore-user-service-test"} 3`,
		`aegiscore_localcache_requests_total{cache="auth_token_version",environment="test",result="hit",service="aegiscore-user-service-test"} 3`,
		`aegiscore_localcache_requests_total{cache="rbac_user_roles",environment="test",result="miss",service="aegiscore-user-service-test"} 2`,
		`aegiscore_localcache_loads_total{cache="auth_token_version",environment="test",result="success",service="aegiscore-user-service-test"} 0`,
		`aegiscore_localcache_evictions_total{cache="auth_token_version",environment="test",service="aegiscore-user-service-test"} 0`,
		`aegiscore_localcache_capacity{cache="auth_token_version",environment="test",service="aegiscore-user-service-test"} 1000`,
		`aegiscore_runtime_component_running{environment="test",resource="rbac_policy_watcher",service="aegiscore-user-service-test"} 1`,
	} {
		assert.Contains(t, body, want)
	}
}

type stubLastError struct {
	err error
}

func (s stubLastError) LastError() error {
	return s.err
}

type stubWatcherStatus struct {
	running bool
	err     error
}

func (s stubWatcherStatus) Running() bool {
	return s.running
}

func (s stubWatcherStatus) LastError() error {
	return s.err
}

type fakePurgeTaskPool struct {
	authredis.PurgeTaskPool
	stats workerpool.Stats
}

func (p fakePurgeTaskPool) Stats() workerpool.Stats {
	return p.stats
}

type fakeLocalcacheStatsSource struct {
	name  string
	stats localcache.Stats
}

func (s fakeLocalcacheStatsSource) Name() string {
	return s.name
}

func (s fakeLocalcacheStatsSource) Stats() localcache.Stats {
	return s.stats
}

func gatherProviderText(t *testing.T, provider *commonmetrics.Provider) string {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	require.NoError(t, err)
	var builder strings.Builder
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			builder.WriteString(family.GetName())
			builder.WriteByte('{')
			for i, label := range metric.GetLabel() {
				if i > 0 {
					builder.WriteByte(',')
				}
				builder.WriteString(label.GetName())
				builder.WriteString(`="`)
				builder.WriteString(label.GetValue())
				builder.WriteByte('"')
			}
			builder.WriteString("} ")
			switch {
			case metric.GetGauge() != nil:
				builder.WriteString(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", metric.GetGauge().GetValue()), "0"), "."))
			case metric.GetCounter() != nil:
				builder.WriteString(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", metric.GetCounter().GetValue()), "0"), "."))
			}
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}
