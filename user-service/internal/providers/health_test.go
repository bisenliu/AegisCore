package providers

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/mattn/go-sqlite3"
	rediscmd "github.com/redis/go-redis/v9"

	"github.com/aegiscore/user-service/internal/router"
)

func TestPostgresHealthChecker(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:health_checker?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	result := postgresHealthChecker{name: "postgres.user_db", db: db}.Check(context.Background())
	if result.Status != router.HealthCheckStatusOK {
		t.Fatalf("result = %#v, want ok", result)
	}

	_ = db.Close()
	result = postgresHealthChecker{name: "postgres.user_db", db: db}.Check(context.Background())
	if result.Status != router.HealthCheckStatusUnavailable || result.Message == "" {
		t.Fatalf("result = %#v, want unavailable", result)
	}
}

func TestRedisHealthChecker(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	result := redisHealthChecker{name: "redis.cache_redis", client: client}.Check(context.Background())
	if result.Status != router.HealthCheckStatusOK {
		t.Fatalf("result = %#v, want ok", result)
	}

	redisServer.Close()
	result = redisHealthChecker{name: "redis.cache_redis", client: client}.Check(context.Background())
	if result.Status != router.HealthCheckStatusUnavailable || result.Message == "" {
		t.Fatalf("result = %#v, want unavailable", result)
	}
}

func TestCasbinPolicyHealthChecker(t *testing.T) {
	checker := casbinPolicyHealthChecker{engine: stubLastError{}}
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
