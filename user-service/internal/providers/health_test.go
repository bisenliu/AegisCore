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

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	"github.com/aegiscore/common/runtime/workerpool"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	"github.com/aegiscore/user-service/internal/router"
)

func TestPostgresHealthChecker(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:health_checker?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	checker := postgresHealthChecker{name: "postgres.user_db", db: db}
	if checker.Name() != "postgres.user_db" {
		t.Fatalf("name = %q, want postgres.user_db", checker.Name())
	}
	result := checker.Check(context.Background())
	if result.Status != router.HealthCheckStatusOK {
		t.Fatalf("result = %#v, want ok", result)
	}

	_ = db.Close()
	result = checker.Check(context.Background())
	if result.Status != router.HealthCheckStatusUnavailable || result.Message == "" {
		t.Fatalf("result = %#v, want unavailable", result)
	}
}

func TestRedisHealthChecker(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	checker := redisHealthChecker{name: "redis.cache_redis", client: client}
	if checker.Name() != "redis.cache_redis" {
		t.Fatalf("name = %q, want redis.cache_redis", checker.Name())
	}
	result := checker.Check(context.Background())
	if result.Status != router.HealthCheckStatusOK {
		t.Fatalf("result = %#v, want ok", result)
	}

	redisServer.Close()
	result = checker.Check(context.Background())
	if result.Status != router.HealthCheckStatusUnavailable || result.Message == "" {
		t.Fatalf("result = %#v, want unavailable", result)
	}
}

func TestCasbinPolicyHealthChecker(t *testing.T) {
	checker := casbinPolicyHealthChecker{engine: stubLastError{}}
	if checker.Name() != "rbac.casbin_policy" {
		t.Fatalf("name = %q, want rbac.casbin_policy", checker.Name())
	}
	if result := checker.Check(context.Background()); result.Status != router.HealthCheckStatusOK {
		t.Fatalf("result = %#v, want ok", result)
	}

	checker = casbinPolicyHealthChecker{engine: stubLastError{err: errors.New("load failed")}}
	result := checker.Check(context.Background())
	if result.Status != router.HealthCheckStatusUnavailable || result.Message == "" {
		t.Fatalf("result = %#v, want unavailable", result)
	}
}

func TestWatcherHealthChecker(t *testing.T) {
	checker := watcherHealthChecker{watcher: stubWatcherStatus{running: true}}
	if checker.Name() != "rbac.policy_watcher" {
		t.Fatalf("name = %q, want rbac.policy_watcher", checker.Name())
	}
	if result := checker.Check(context.Background()); result.Status != router.HealthCheckStatusOK {
		t.Fatalf("result = %#v, want ok", result)
	}

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
			if result.Status != router.HealthCheckStatusUnavailable || result.Message == "" {
				t.Fatalf("result = %#v, want unavailable", result)
			}
		})
	}
}

func TestRegisterRuntimeDependencyMetricsRegistersCollectors(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:runtime_metrics?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	cfg := &config.Config{
		App: config.AppConfig{Name: "aegiscore-user-service-test", Environment: "test"},
		Redis: map[string]config.RedisConfig{
			"cache_redis": {PingTimeout: time.Second},
		},
		Observability: config.ObservabilityConfig{
			Metrics: config.MetricsConfig{Enabled: true, Path: "/metrics"},
		},
	}
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      cfg.Observability.Metrics,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	if err := RegisterRuntimeDependencyMetrics(RuntimeDependencyMetricsParams{
		Config:           cfg,
		Metrics:          provider,
		UserDB:           db,
		CacheRedis:       client,
		SessionPurgePool: fakePurgeTaskPool{stats: workerpool.Stats{Name: "auth.redis.session_purge", Workers: 4, Submitted: 3}},
		PolicyWatcher:    stubWatcherStatus{running: true},
		AuthTokenCache:   fakeLocalcacheStatsSource{name: "auth_token_version", stats: localcache.Stats{Hit: 3, Capacity: 1000}},
		RBACRolesCache:   fakeLocalcacheStatsSource{name: "rbac_user_roles", stats: localcache.Stats{Miss: 2, Capacity: 2000}},
	}); err != nil {
		t.Fatalf("RegisterRuntimeDependencyMetrics: %v", err)
	}

	body := gatherProviderText(t, provider)
	for _, want := range []string{
		`aegiscore_postgres_pool_open_connections{environment="test",resource="user_db",service="aegiscore-user-service-test"}`,
		`aegiscore_redis_up{environment="test",resource="cache_redis",service="aegiscore-user-service-test"} 1`,
		`aegiscore_workerpool_tasks_total{environment="test",event="submitted",pool="auth_session_purge_pool",service="aegiscore-user-service-test"} 3`,
		`aegiscore_localcache_requests_total{cache="auth_token_version",environment="test",result="hit",service="aegiscore-user-service-test"} 3`,
		`aegiscore_localcache_requests_total{cache="rbac_user_roles",environment="test",result="miss",service="aegiscore-user-service-test"} 2`,
		`aegiscore_runtime_component_running{environment="test",resource="rbac_policy_watcher",service="aegiscore-user-service-test"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
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
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
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
