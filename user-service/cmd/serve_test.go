package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testContextKey string

func TestRunServeStopContextPreservesUpstreamValuesWithoutCancellation(t *testing.T) {
	key := testContextKey("trace-id")
	parent := context.WithValue(context.Background(), key, "test-trace")
	ctx, cancel := context.WithCancel(parent)
	configPath := writeServeTestConfig(t, 2*time.Second, 4*time.Second)

	appFactory := func(configPath string) lifecycleApp {
		require.NotEmpty(t, configPath)

		return testLifecycleApp{
			start: func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				remaining := time.Until(deadline)
				require.Positive(t, remaining)
				require.LessOrEqual(t, remaining, 2*time.Second)
				cancel()
				return nil
			},
			stop: func(ctx context.Context) error {
				require.Equal(t, "test-trace", ctx.Value(key))
				require.NoError(t, ctx.Err())
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				remaining := time.Until(deadline)
				require.Positive(t, remaining)
				require.LessOrEqual(t, remaining, 4*time.Second)
				return nil
			},
		}
	}

	require.NoError(t, runServe(ctx, configPath, appFactory))
}

func TestRunServeRejectsInvalidConfigBeforeCreatingApp(t *testing.T) {
	called := false
	appFactory := func(string) lifecycleApp {
		called = true
		return testLifecycleApp{}
	}

	err := runServe(context.Background(), writeServeTestConfig(t, 0, time.Second), appFactory)
	require.ErrorContains(t, err, "runtime.lifecycle.start_timeout must be > 0")
	require.False(t, called)
}

func writeServeTestConfig(t testing.TB, startTimeout time.Duration, stopTimeout time.Duration) string {
	t.Helper()
	content := `app:
  name: aegiscore-test
  environment: local
runtime:
  lifecycle:
    start_timeout: ` + startTimeout.String() + `
    stop_timeout: ` + stopTimeout.String() + `
server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 18080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 1s
  grpc:
    enabled: false
    shutdown_timeout: 1s
auth:
  jwt:
    secret: secret-123456789012345678901234567890
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
    password_change_token_ttl: 5m
  password_kdf:
    argon2_concurrency: 2
    argon2_queue_size: 16
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5
rbac: {}
ent:
  sql_debug: false
log:
  level: info
  format: json
observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: false
resources:
  redis:
    cache_redis:
      addr: 127.0.0.1:6379
  postgres:
    user_db:
      host: 127.0.0.1
      port: 15432
      username: aegiscore
      password: ""
      db_name: aegiscore_user
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
