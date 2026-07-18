package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestFxLifecycleStopsCriticalResourcesInReverseOrder(t *testing.T) {
	var order []string
	app := fx.New(
		fx.NopLogger,
		testLifecycleHook("logger", &order, nil),
		testLifecycleHook("tracing", &order, nil),
		testLifecycleHook("postgres", &order, nil),
		testLifecycleHook("redis", &order, nil),
		testLifecycleHook("ent", &order, nil),
		testLifecycleHook("feature_worker_cache", &order, nil),
		testLifecycleHook("rbac_watcher", &order, nil),
		testLifecycleHook("pprof", &order, nil),
		testLifecycleHook("http", &order, nil),
	)
	require.NoError(t, app.Start(context.Background()))
	require.NoError(t, app.Stop(context.Background()))

	require.Equal(t, []string{
		"http",
		"pprof",
		"rbac_watcher",
		"feature_worker_cache",
		"ent",
		"redis",
		"postgres",
		"tracing",
		"logger",
	}, order)
}

func TestFxLifecycleContinuesStopHooksAfterOrdinaryError(t *testing.T) {
	stopErr := errors.New("stop failed")
	var order []string
	app := fx.New(
		fx.NopLogger,
		testLifecycleHook("logger", &order, nil),
		testLifecycleHook("tracing", &order, nil),
		testLifecycleHook("http", &order, stopErr),
	)
	require.NoError(t, app.Start(context.Background()))

	err := app.Stop(context.Background())
	require.ErrorIs(t, err, stopErr)
	require.Equal(t, []string{"http", "tracing", "logger"}, order)
}

func TestFxLifecycleStopHooksShareDeadline(t *testing.T) {
	remainingSeen := make(chan time.Duration, 1)
	app := fx.New(
		fx.NopLogger,
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(context.Context) error { return nil },
				OnStop: func(ctx context.Context) error {
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					remainingSeen <- time.Until(deadline)
					return nil
				},
			})
			lifecycle.Append(fx.Hook{
				OnStart: func(context.Context) error { return nil },
				OnStop: func(context.Context) error {
					time.Sleep(30 * time.Millisecond)
					return nil
				},
			})
		}),
	)
	require.NoError(t, app.Start(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.NoError(t, app.Stop(ctx))
	remaining := <-remainingSeen
	require.Less(t, remaining, 80*time.Millisecond)
	require.Greater(t, remaining, time.Duration(0))
}

func testLifecycleHook(name string, order *[]string, stopErr error) fx.Option {
	return fx.Invoke(func(lifecycle fx.Lifecycle) {
		lifecycle.Append(fx.Hook{
			OnStart: func(context.Context) error { return nil },
			OnStop: func(context.Context) error {
				*order = append(*order, name)
				return stopErr
			},
		})
	})
}
