package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
)

type testContextKey string

func TestRunServeStopContextPreservesUpstreamValuesWithoutCancellation(t *testing.T) {
	key := testContextKey("trace-id")
	parent := context.WithValue(context.Background(), key, "test-trace")
	ctx, cancel := context.WithCancel(parent)

	appFactory := func(configPath string) lifecycleApp {
		require.Equal(t, "test-config.yaml", configPath)

		return testLifecycleApp{
			start: func(_ context.Context) error {
				cancel()
				return nil
			},
			stop: func(ctx context.Context) error {
				require.Equal(t, "test-trace", ctx.Value(key))
				require.NoError(t, ctx.Err())
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				remaining := time.Until(deadline)
				require.Greater(t, remaining, time.Duration(0))
				require.LessOrEqual(t, remaining, fxAppStopTimeout)
				return nil
			},
		}
	}

	require.NoError(t, runServe(ctx, "test-config.yaml", appFactory))
}

func TestFxAppLifecycleTimeouts(t *testing.T) {
	require.Equal(t, 15*time.Second, fxAppStartTimeout)

	cfg := config.DefaultConfig()
	require.GreaterOrEqual(t, fxAppStopTimeout, cfg.Server.HTTP.ShutdownTimeout)
}
