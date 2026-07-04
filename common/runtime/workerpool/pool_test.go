package workerpool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestPoolLimitsConcurrency(t *testing.T) {
	pool := newTestPool(t, Options{Name: "test-concurrency", Workers: 2})
	defer stopTestPool(t, pool)

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var running atomic.Int64
	var maxRunning atomic.Int64
	for i := 0; i < 2; i++ {
		err := pool.Submit(context.Background(), Task{
			Name: "blocking",
			Run: func(context.Context) error {
				current := running.Add(1)
				updateMax(&maxRunning, current)
				started <- struct{}{}
				<-release
				running.Add(-1)
				return nil
			},
		})
		require.NoError(t, err)
	}

	waitForCount(t, started, 2)
	close(release)
	waitForPool(t, pool, func(stats Stats) bool { return stats.Completed == 2 })

	require.LessOrEqual(t, maxRunning.Load(), int64(2))
}

func TestPoolSubmitWaitsWhenWorkersAreBusy(t *testing.T) {
	pool := newTestPool(t, Options{Name: "test-submit-waits", Workers: 1})
	defer stopTestPool(t, pool)

	release := make(chan struct{})
	require.NoError(t, pool.Submit(context.Background(), Task{Name: "running", Run: waitTask(release)}))
	waitForPool(t, pool, func(stats Stats) bool { return stats.Running == 1 })

	accepted := make(chan error, 1)
	go func() {
		accepted <- pool.Submit(context.Background(), Task{Name: "waiting", Run: func(context.Context) error { return nil }})
	}()
	waitForPool(t, pool, func(stats Stats) bool { return stats.Waiting == 1 && stats.Queued == 1 })

	close(release)
	select {
	case err := <-accepted:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Submit did not return after worker became available")
	}
	waitForPool(t, pool, func(stats Stats) bool { return stats.Completed == 2 && stats.Rejected == 0 })
}

func TestPoolTaskErrorIsObservable(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	pool := newTestPoolWithLogger(t, zap.New(core), Options{Name: "test-error", Workers: 1})
	defer stopTestPool(t, pool)

	taskErr := errors.New("task failed")
	err := pool.Submit(context.Background(), Task{
		Name: "error-task",
		Run:  func(context.Context) error { return taskErr },
	})
	require.NoError(t, err)
	waitForPool(t, pool, func(stats Stats) bool { return stats.Failed == 1 })

	require.Zero(t, pool.Stats().Completed)
	require.Equal(t, 1, logs.FilterMessage("worker pool task failed").Len())
}

func TestPoolTaskPanicIsRecovered(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	pool := newTestPoolWithLogger(t, zap.New(core), Options{Name: "test-panic", Workers: 1})
	defer stopTestPool(t, pool)

	err := pool.Submit(context.Background(), Task{
		Name: "panic-task",
		Run: func(context.Context) error {
			panic("boom")
		},
	})
	require.NoError(t, err)
	waitForPool(t, pool, func(stats Stats) bool { return stats.Panicked == 1 })

	require.Equal(t, 1, logs.FilterMessage("worker pool task panicked").Len())
}

func TestPoolTaskReceivesSubmitContextCancellation(t *testing.T) {
	pool := newTestPool(t, Options{Name: "test-submit-context", Workers: 1})
	defer stopTestPool(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	taskDone := make(chan error, 1)
	err := pool.Submit(ctx, Task{
		Name: "cancel-by-submit-context",
		Run: func(taskCtx context.Context) error {
			close(started)
			<-taskCtx.Done()
			err := taskCtx.Err()
			taskDone <- err
			return err
		},
	})
	require.NoError(t, err)
	waitForCount(t, started, 1)

	cancel()

	select {
	case err := <-taskDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("task did not observe submit context cancellation")
	}
	waitForPool(t, pool, func(stats Stats) bool { return stats.Failed == 1 })
}

func TestPoolSubmitAfterStopReturnsClosed(t *testing.T) {
	pool := newTestPool(t, Options{Name: "test-closed", Workers: 1})
	stopTestPool(t, pool)

	err := pool.Submit(context.Background(), Task{Name: "after-stop", Run: func(context.Context) error { return nil }})

	require.ErrorIs(t, err, ErrClosed)
}

func TestPoolRegistersLifecycleStopHook(t *testing.T) {
	lifecycle := fxtest.NewLifecycle(t)
	pool, err := New(lifecycle, zap.NewNop(), Options{Name: "test-lifecycle", Workers: 1})
	require.NoError(t, err)
	require.NoError(t, pool.Submit(context.Background(), Task{Name: "noop", Run: func(context.Context) error { return nil }}))

	lifecycle.RequireStart()
	lifecycle.RequireStop()

	err = pool.Submit(context.Background(), Task{Name: "after-lifecycle-stop", Run: func(context.Context) error { return nil }})
	require.ErrorIs(t, err, ErrClosed)
}

func TestPoolStopWaitsForAcceptedTasks(t *testing.T) {
	pool := newTestPool(t, Options{Name: "test-stop-waits", Workers: 2})
	release := make(chan struct{})
	var completed atomic.Int64
	for i := 0; i < 2; i++ {
		err := pool.Submit(context.Background(), Task{
			Name: "accepted",
			Run: func(context.Context) error {
				<-release
				completed.Add(1)
				return nil
			},
		})
		require.NoError(t, err)
	}
	waitForPool(t, pool, func(stats Stats) bool { return stats.Running == 2 })

	done := make(chan error, 1)
	go func() {
		done <- pool.Stop(context.Background())
	}()
	require.Zero(t, completed.Load())
	close(release)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Stop did not return")
	}
	require.Equal(t, int64(2), completed.Load())
}

func TestPoolStopDoesNotWaitForBlockedSubmitLock(t *testing.T) {
	pool := newTestPool(t, Options{Name: "test-stop-during-blocked-submit", Workers: 1})
	release := make(chan struct{})
	require.NoError(t, pool.Submit(context.Background(), Task{Name: "running", Run: waitTask(release)}))
	waitForPool(t, pool, func(stats Stats) bool { return stats.Running == 1 })

	submitDone := make(chan error, 1)
	go func() {
		submitDone <- pool.Submit(context.Background(), Task{Name: "waiting", Run: func(context.Context) error { return nil }})
	}()
	waitForPool(t, pool, func(stats Stats) bool { return stats.Waiting == 1 })

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- pool.Stop(context.Background())
	}()
	close(release)

	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Stop did not return")
	}
	select {
	case err := <-submitDone:
		require.True(t, err == nil || errors.Is(err, ErrClosed), "blocked Submit err = %v, want nil or ErrClosed", err)
	case <-time.After(time.Second):
		t.Fatal("blocked Submit did not return")
	}
}

func TestPoolStopHonorsContextTimeout(t *testing.T) {
	pool := newTestPool(t, Options{Name: "test-stop-timeout", Workers: 1})
	err := pool.Submit(context.Background(), Task{
		Name: "blocked",
		Run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	require.NoError(t, err)
	waitForPool(t, pool, func(stats Stats) bool { return stats.Running == 1 })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = pool.Stop(ctx)

	require.Error(t, err)
}

func newTestPool(t *testing.T, opts Options) *Pool {
	t.Helper()
	return newTestPoolWithLogger(t, zap.NewNop(), opts)
}

func newTestPoolWithLogger(t *testing.T, log *zap.Logger, opts Options) *Pool {
	t.Helper()
	pool, err := NewUnmanaged(log, opts)
	require.NoError(t, err)
	return pool
}

func stopTestPool(t *testing.T, pool *Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, pool.Stop(ctx))
}

func waitTask(release <-chan struct{}) func(context.Context) error {
	return func(context.Context) error {
		<-release
		return nil
	}
}

func waitForCount(t *testing.T, ch <-chan struct{}, count int) {
	t.Helper()
	timeout := time.After(time.Second)
	for i := 0; i < count; i++ {
		select {
		case <-ch:
		case <-timeout:
			t.Fatalf("received %d events, want %d", i, count)
		}
	}
}

func waitForPool(t *testing.T, pool *Pool, condition func(Stats) bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		return condition(pool.Stats())
	}, time.Second, 5*time.Millisecond, "condition not met; stats=%+v", pool.Stats())
}

func updateMax(max *atomic.Int64, value int64) {
	for {
		current := max.Load()
		if value <= current || max.CompareAndSwap(current, value) {
			return
		}
	}
}
