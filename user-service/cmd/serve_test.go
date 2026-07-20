package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

type testContextKey string

func TestRunServeStopContextPreservesUpstreamValuesWithoutCancellation(t *testing.T) {
	key := testContextKey("trace-id")
	parent := context.WithValue(context.Background(), key, "test-trace")
	ctx, cancel := context.WithCancel(parent)
	configPath := writeServeTestConfig(t, 2*time.Second, 45*time.Second)

	appFactory := func(cfg *serviceconfig.Config) lifecycleApp {
		require.NotNil(t, cfg)
		require.Equal(t, 2*time.Second, cfg.Runtime.Lifecycle.StartTimeout)
		require.Equal(t, 45*time.Second, cfg.Runtime.Lifecycle.StopTimeout)

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
				require.LessOrEqual(t, remaining, 45*time.Second)
				return nil
			},
		}
	}

	require.NoError(t, runServe(ctx, configPath, appFactory))
}

func TestRunServeRejectsInvalidConfigBeforeCreatingApp(t *testing.T) {
	called := false
	appFactory := func(*serviceconfig.Config) lifecycleApp {
		called = true
		return testLifecycleApp{}
	}

	err := runServe(context.Background(), writeServeTestConfig(t, 0, 45*time.Second), appFactory)
	require.ErrorContains(t, err, "runtime.lifecycle.start_timeout must be > 0")
	require.False(t, called)
}

func TestRunServeHandlesInternalShutdownSignal(t *testing.T) {
	stopErr := errors.New("stop failed")
	tests := []struct {
		name          string
		exitCode      int
		stopErr       error
		wantExitError bool
		wantStopError bool
	}{
		{name: "zero exit code"},
		{name: "non-zero exit code", exitCode: 23, wantExitError: true},
		{name: "stop error", stopErr: stopErr, wantStopError: true},
		{name: "exit code and stop error", exitCode: 42, stopErr: stopErr, wantExitError: true, wantStopError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdownSignals := make(chan fx.ShutdownSignal, 1)
			shutdownSignals <- fx.ShutdownSignal{ExitCode: tt.exitCode}
			var stopCalls int
			appFactory := func(*serviceconfig.Config) lifecycleApp {
				return testLifecycleApp{
					start: func(context.Context) error { return nil },
					wait:  shutdownSignals,
					stop: func(context.Context) error {
						stopCalls++
						return tt.stopErr
					},
				}
			}

			err := runServe(context.Background(), writeServeTestConfig(t, time.Second, 45*time.Second), appFactory)
			require.Equal(t, 1, stopCalls)
			if tt.wantExitError {
				require.ErrorContains(t, err, "exit code "+fmt.Sprint(tt.exitCode))
			} else if !tt.wantStopError {
				require.NoError(t, err)
			}
			if tt.wantStopError {
				require.ErrorIs(t, err, stopErr)
			}
		})
	}
}

func TestRunServeReturnsExternalShutdownStopError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stopErr := errors.New("external stop failed")
	appFactory := func(*serviceconfig.Config) lifecycleApp {
		return testLifecycleApp{
			start: func(context.Context) error {
				cancel()
				return nil
			},
			stop: func(context.Context) error { return stopErr },
		}
	}

	err := runServe(ctx, writeServeTestConfig(t, time.Second, 45*time.Second), appFactory)
	require.ErrorIs(t, err, stopErr)
}

func TestRunServeConcurrentExitSourcesStopOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	shutdownSignals := make(chan fx.ShutdownSignal, 2)
	configPath := writeServeTestConfig(t, time.Second, 45*time.Second)
	var stopCalls atomic.Int32
	appFactory := func(*serviceconfig.Config) lifecycleApp {
		return testLifecycleApp{
			start: func(context.Context) error {
				cancel()
				shutdownSignals <- fx.ShutdownSignal{}
				shutdownSignals <- fx.ShutdownSignal{}
				return nil
			},
			wait: shutdownSignals,
			stop: func(context.Context) error {
				stopCalls.Add(1)
				return nil
			},
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, configPath, appFactory)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runServe did not return after concurrent exit sources")
	}
	require.Equal(t, int32(1), stopCalls.Load())
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
    primary_db:
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
