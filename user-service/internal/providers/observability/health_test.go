package observability

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
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
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
	checker := casbinPolicyHealthChecker{engine: stubPolicyHealth{status: readyPolicyProjectionStatus(2, 2)}}
	require.Equal(t, "rbac.casbin_policy", checker.Name())
	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)

	tests := []struct {
		name   string
		status permissionapplication.PolicyProjectionStatus
	}{
		{name: "uninitialized"},
		{name: "last reload failed", status: permissionapplication.PolicyProjectionStatus{Initialized: true, AppliedRevision: 2, TargetRevision: 2, LastError: errors.New("load failed")}},
		{name: "reload status failed", status: permissionapplication.PolicyProjectionStatus{Initialized: true, AppliedRevision: 2, TargetRevision: 2}},
		{name: "target not reached", status: readyPolicyProjectionStatus(2, 3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := (casbinPolicyHealthChecker{engine: stubPolicyHealth{status: tt.status}}).Check(context.Background())
			require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)
			require.NotEmpty(t, result.Message)
		})
	}
}

func TestCasbinPolicyHealthCheckerRecoversAfterReloadSuccess(t *testing.T) {
	engine := &stubPolicyHealth{status: permissionapplication.PolicyProjectionStatus{LastError: errors.New("initial load failed")}}
	checker := casbinPolicyHealthChecker{engine: engine}

	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)

	engine.status = readyPolicyProjectionStatus(4, 4)
	result = checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)
}

func TestWatcherHealthChecker(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	healthy := permissionapplication.PolicyWatcherStatusSnapshot{
		Running: true, SubscriptionState: permissionapplication.PolicyWatcherSubscriptionReconnecting,
		LastReconcileSuccessAt: now.Add(-45 * time.Second), LastFailureAt: now.Add(-time.Second),
		SubscriptionErrorCategory: permissionapplication.PolicyWatcherErrorReceive,
	}
	checker := watcherHealthChecker{watcher: stubWatcherStatus{status: healthy}, maxStaleness: 45 * time.Second, now: func() time.Time { return now }}
	require.Equal(t, "rbac.policy_watcher", checker.Name())
	result := checker.Check(context.Background())
	require.Equal(t, router.HealthCheckStatusOK, result.Status)

	cases := []struct {
		name    string
		status  permissionapplication.PolicyWatcherStatusSnapshot
		message string
	}{
		{name: "stopped", message: "rbac policy watcher stopped"},
		{name: "never synchronized", status: permissionapplication.PolicyWatcherStatusSnapshot{Running: true}, message: "rbac policy watcher not synchronized"},
		{name: "stale", status: permissionapplication.PolicyWatcherStatusSnapshot{Running: true, LastReconcileSuccessAt: now.Add(-46 * time.Second)}, message: "rbac policy watcher stale"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := watcherHealthChecker{watcher: stubWatcherStatus{status: tt.status}, maxStaleness: 45 * time.Second, now: func() time.Time { return now }}.Check(context.Background())
			require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)
			require.Equal(t, tt.message, result.Message)
		})
	}
}

func TestOutboxDispatcherHealthChecker(t *testing.T) {
	checker := outboxDispatcherHealthChecker{dispatcher: &stubDispatcherStatus{status: permissionapplication.DispatcherStatus{Running: true, DueCount: 3, LastErrorCategory: permissionapplication.DispatcherErrorPublish}}}
	require.Equal(t, "rbac.outbox_dispatcher", checker.Name())
	require.Equal(t, router.HealthCheckResult{Name: checker.Name(), Status: router.HealthCheckStatusOK}, checker.Check(context.Background()))
	require.Equal(t, 1, checker.dispatcher.(*stubDispatcherStatus).calls)

	tests := []struct {
		name       string
		dispatcher permissionapplication.OutboxDispatcherStatus
		message    string
	}{
		{name: "unavailable", message: "rbac outbox dispatcher unavailable"},
		{name: "not running", dispatcher: &stubDispatcherStatus{}, message: "rbac outbox dispatcher not running"},
		{name: "query error", dispatcher: &stubDispatcherStatus{err: errors.New("query failed")}, message: "rbac outbox dispatcher status query failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := outboxDispatcherHealthChecker{dispatcher: tt.dispatcher}.Check(context.Background())
			require.Equal(t, router.HealthCheckStatusUnavailable, result.Status)
			require.Equal(t, tt.message, result.Message)
		})
	}
}

func TestProvideHealthChecksUsesResourcePingTimeouts(t *testing.T) {
	settings := serviceconfig.ResourceSettings{Redis: commonresources.RedisConfigs{
		"cache_redis": {Mode: commonresources.RedisModeCluster, Addrs: []string{"127.0.0.1:6379"}, Timeout: 2 * time.Second},
	}}
	checks := ProvideHealthChecks(HealthCheckParams{Resources: settings, RBAC: serviceconfig.RBACSettings{PolicyWatcher: serviceconfig.DefaultPolicyWatcherConfig()}})

	require.Len(t, checks.Readiness, 5)
	postgres, ok := checks.Readiness[0].(postgresHealthChecker)
	require.True(t, ok)
	require.Equal(t, commonresources.DefaultPostgresPingTimeout(), postgres.timeout)
	redis, ok := checks.Readiness[1].(redisHealthChecker)
	require.True(t, ok)
	require.Equal(t, 2*time.Second, redis.timeout)
	watcher := checks.Readiness[3].(watcherHealthChecker)
	require.Equal(t, 45*time.Second, watcher.maxStaleness)
}

func TestProvideHealthChecksAppliesDefaultRedisPingTimeout(t *testing.T) {
	settings := serviceconfig.ResourceSettings{Redis: commonresources.RedisConfigs{
		"cache_redis": {Mode: commonresources.RedisModeCluster, Addrs: []string{"127.0.0.1:6379"}},
	}}
	checks := ProvideHealthChecks(HealthCheckParams{Resources: settings, RBAC: serviceconfig.RBACSettings{PolicyWatcher: serviceconfig.DefaultPolicyWatcherConfig()}})

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
		RBAC:             serviceconfig.RBACSettings{PolicyWatcher: serviceconfig.DefaultPolicyWatcherConfig()},
		Metrics:          provider,
		PrimaryDB:        db,
		CacheRedis:       client,
		SessionPurgePool: fakePurgeTaskPool{stats: workerpool.Stats{Name: "auth.redis.session_purge", Workers: 5, Submitted: 3}},
		PolicyWatcher: stubWatcherStatus{status: permissionapplication.PolicyWatcherStatusSnapshot{
			Running: true, SubscriptionState: permissionapplication.PolicyWatcherSubscriptionConnected,
			LastSubscriptionSuccessAt: time.Unix(1_722_770_390, 0), LastReconcileSuccessAt: time.Now().Add(-10 * time.Second), ReconnectAttempts: 2,
		}},
		AuthTokenCache: fakeLocalcacheStatsSource{name: "auth_token_version", stats: localcache.Stats{Hit: 3, Capacity: 1000}},
		RBACRolesCache: fakeLocalcacheStatsSource{name: "rbac_user_roles", stats: localcache.Stats{Miss: 2, Capacity: 2000}},
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
		`aegiscore_localcache_capacity_evictions_total{cache="auth_token_version",environment="test",service="aegiscore-user-service-test"} 0`,
		`aegiscore_localcache_capacity{cache="auth_token_version",environment="test",service="aegiscore-user-service-test"} 1000`,
		`aegiscore_user_service_rbac_policy_watcher_running{environment="test",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_rbac_policy_watcher_subscription_state{environment="test",service="aegiscore-user-service-test",state="starting"} 0`,
		`aegiscore_user_service_rbac_policy_watcher_subscription_state{environment="test",service="aegiscore-user-service-test",state="connected"} 1`,
		`aegiscore_user_service_rbac_policy_watcher_subscription_state{environment="test",service="aegiscore-user-service-test",state="reconnecting"} 0`,
		`aegiscore_user_service_rbac_policy_watcher_subscription_state{environment="test",service="aegiscore-user-service-test",state="stopped"} 0`,
		`aegiscore_user_service_rbac_policy_watcher_last_subscription_success_timestamp_seconds{environment="test",service="aegiscore-user-service-test"} 1722770390`,
		`aegiscore_user_service_rbac_policy_watcher_max_staleness_seconds{environment="test",service="aegiscore-user-service-test"} 45`,
		`aegiscore_user_service_rbac_policy_watcher_reconnect_attempts_total{environment="test",service="aegiscore-user-service-test"} 2`,
	} {
		assert.Contains(t, body, want)
	}
	assert.NotContains(t, body, `aegiscore_runtime_component_running{environment="test",resource="rbac_policy_watcher"`)
	assert.NotContains(t, body, `aegiscore_runtime_component_last_error{environment="test",resource="rbac_policy_watcher"`)
}

func TestRegisterRuntimeDependencyMetricsSkipsDisabledProvider(t *testing.T) {
	require.NoError(t, RegisterRuntimeDependencyMetrics(RuntimeDependencyMetricsParams{}))
}

type stubPolicyHealth struct {
	status permissionapplication.PolicyProjectionStatus
}

func (s stubPolicyHealth) ProjectionStatus() permissionapplication.PolicyProjectionStatus {
	return s.status
}

func readyPolicyProjectionStatus(appliedRevision int64, targetRevision int64) permissionapplication.PolicyProjectionStatus {
	return permissionapplication.PolicyProjectionStatus{
		Initialized:     true,
		ReloadSucceeded: true,
		AppliedRevision: appliedRevision,
		TargetRevision:  targetRevision,
	}
}

type stubWatcherStatus struct {
	status permissionapplication.PolicyWatcherStatusSnapshot
}

type stubDispatcherStatus struct {
	status permissionapplication.DispatcherStatus
	err    error
	calls  int
}

func (s *stubDispatcherStatus) Status(context.Context) (permissionapplication.DispatcherStatus, error) {
	s.calls++
	return s.status, s.err
}

func (s stubWatcherStatus) Status() permissionapplication.PolicyWatcherStatusSnapshot {
	return s.status
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
