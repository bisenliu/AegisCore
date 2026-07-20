package localcache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewLoadingCacheValidatesConfig(t *testing.T) {
	base := Config{Name: "test", Capacity: 10, TTL: time.Second, LoadTimeout: time.Second}
	loader := func(context.Context, string) (int, error) { return 1, nil }
	tests := []struct {
		name string
		cfg  Config
		load Loader[string, int]
		want error
	}{
		{name: "missing name", cfg: Config{Capacity: 10, TTL: time.Second, LoadTimeout: time.Second}, load: loader, want: ErrNameRequired},
		{name: "missing capacity", cfg: Config{Name: "test", TTL: time.Second, LoadTimeout: time.Second}, load: loader, want: ErrCapacityRequired},
		{name: "missing ttl", cfg: Config{Name: "test", Capacity: 10, LoadTimeout: time.Second}, load: loader, want: ErrTTLRequired},
		{name: "missing load timeout", cfg: Config{Name: "test", Capacity: 10, TTL: time.Second}, load: loader, want: ErrLoadTimeoutRequired},
		{name: "missing loader", cfg: base, load: nil, want: ErrLoaderRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewLoadingCache[string, int](tt.cfg, tt.load)
			require.ErrorIs(t, err, tt.want)
			require.Nil(t, cache)
		})
	}
}

func TestLoadingCacheUsesFixedTTLWithoutTouchOnHit(t *testing.T) {
	var loads atomic.Uint64
	cache := newTestCache(t, Config{Name: "test", Capacity: 10, TTL: 80 * time.Millisecond, LoadTimeout: time.Second}, func(context.Context, string) (int, error) {
		return int(loads.Add(1)), nil
	})

	first, err := cache.GetOrLoad(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, 1, first)
	require.Eventually(t, func() bool {
		value, loadErr := cache.GetOrLoad(context.Background(), "key")
		return loadErr == nil && value == 1 && cache.Stats().Hit > 0
	}, 40*time.Millisecond, 5*time.Millisecond)
	require.Eventually(t, func() bool {
		value, loadErr := cache.GetOrLoad(context.Background(), "key")
		return loadErr == nil && value == 2
	}, time.Second, 5*time.Millisecond, "读取不应延长首次写入的 TTL")
}

func TestLoadingCacheEnforcesItemCapacity(t *testing.T) {
	loads := make(map[string]int)
	var mu sync.Mutex
	cache := newTestCache(t, Config{Name: "test", Capacity: 1, TTL: time.Minute, LoadTimeout: time.Second}, func(_ context.Context, key string) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		loads[key]++
		return loads[key], nil
	})

	_, err := cache.GetOrLoad(context.Background(), "a")
	require.NoError(t, err)
	_, err = cache.GetOrLoad(context.Background(), "b")
	require.NoError(t, err)
	value, err := cache.GetOrLoad(context.Background(), "a")
	require.NoError(t, err)
	require.Equal(t, 2, value, "最早的 item 应因容量达到上限被移除")
	require.Eventually(t, func() bool { return cache.Stats().Evicted >= 1 }, time.Second, time.Millisecond)
}

func TestLoadingCacheCoalescesConcurrentMisses(t *testing.T) {
	var loads atomic.Uint64
	loaderEntered := make(chan struct{})
	releaseLoader := make(chan struct{})
	cache := newTestCache(t, testConfig(), func(_ context.Context, key string) (int, error) {
		if loads.Add(1) == 1 {
			close(loaderEntered)
		}
		<-releaseLoader
		return len(key), nil
	})

	const callers = 20
	start := make(chan struct{})
	results := make(chan int, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := cache.GetOrLoad(context.Background(), "alice")
			results <- value
			errs <- err
		}()
	}
	close(start)
	waitForSignal(t, loaderEntered)
	require.Eventually(t, func() bool { return cache.Stats().Miss == callers }, time.Second, time.Millisecond)
	close(releaseLoader)
	wg.Wait()

	for range callers {
		require.NoError(t, <-errs)
		require.Equal(t, 5, <-results)
	}
	require.EqualValues(t, 1, loads.Load())
	require.EqualValues(t, 1, cache.Stats().LoadSuccess)
	require.Zero(t, cache.Stats().Hit)
}

func TestLoadingCacheKeepsDifferentKeysIndependent(t *testing.T) {
	entered := make(chan string, 2)
	release := make(chan struct{})
	cache := newTestCache(t, testConfig(), func(_ context.Context, key string) (string, error) {
		entered <- key
		<-release
		return key, nil
	})

	errs := make(chan error, 2)
	for _, key := range []string{"a", "b"} {
		go func() {
			_, err := cache.GetOrLoad(context.Background(), key)
			errs <- err
		}()
	}
	waitForSignal(t, entered)
	waitForSignal(t, entered)
	close(release)
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.EqualValues(t, 2, cache.Stats().LoadSuccess)
}

func TestLoadingCacheDoesNotCacheLoaderErrors(t *testing.T) {
	loadErr := errors.New("load failed")
	var loads atomic.Uint64
	cache := newTestCache(t, testConfig(), func(context.Context, string) (int, error) {
		loads.Add(1)
		return 0, loadErr
	})

	for range 2 {
		_, err := cache.GetOrLoad(context.Background(), "key")
		require.ErrorIs(t, err, loadErr)
	}
	require.EqualValues(t, 2, loads.Load())
	require.EqualValues(t, 2, cache.Stats().LoadError)
	require.Zero(t, cache.Stats().LoadSuccess)
}

func TestLoadingCacheCallerCancellationDoesNotCancelSharedLoader(t *testing.T) {
	type contextKey struct{}
	loaderEntered := make(chan struct{})
	releaseLoader := make(chan struct{})
	cache := newTestCache(t, testConfig(), func(ctx context.Context, _ string) (string, error) {
		close(loaderEntered)
		require.Equal(t, "request-value", ctx.Value(contextKey{}))
		<-releaseLoader
		return "loaded", ctx.Err()
	})

	leaderCtx, cancelLeader := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "request-value"))
	leaderResult := make(chan error, 1)
	go func() {
		_, err := cache.GetOrLoad(leaderCtx, "key")
		leaderResult <- err
	}()
	waitForSignal(t, loaderEntered)

	followerResult := make(chan error, 1)
	go func() {
		value, err := cache.GetOrLoad(context.Background(), "key")
		if err == nil && value != "loaded" {
			err = errors.New("unexpected value")
		}
		followerResult <- err
	}()
	require.Eventually(t, func() bool { return cache.Stats().Miss == 2 }, time.Second, time.Millisecond)
	cancelLeader()
	require.ErrorIs(t, <-leaderResult, context.Canceled)
	close(releaseLoader)
	require.NoError(t, <-followerResult)
	require.EqualValues(t, 1, cache.Stats().LoadSuccess)
}

func TestLoadingCacheDeleteAndClearInvalidateSynchronously(t *testing.T) {
	var loads atomic.Uint64
	cache := newTestCache(t, testConfig(), func(context.Context, string) (uint64, error) {
		return loads.Add(1), nil
	})

	_, err := cache.GetOrLoad(context.Background(), "a")
	require.NoError(t, err)
	_, err = cache.GetOrLoad(context.Background(), "b")
	require.NoError(t, err)
	require.NoError(t, cache.Delete("a"))
	value, err := cache.GetOrLoad(context.Background(), "a")
	require.NoError(t, err)
	require.EqualValues(t, 3, value)
	require.NoError(t, cache.Clear())
	value, err = cache.GetOrLoad(context.Background(), "b")
	require.NoError(t, err)
	require.EqualValues(t, 4, value)
	require.Zero(t, cache.Stats().Evicted, "显式失效不得计入自动驱逐")
}

func TestLoadingCacheCloseRejectsOperationsAndSkipsInflightWrite(t *testing.T) {
	loaderEntered := make(chan struct{})
	releaseLoader := make(chan struct{})
	cache := newTestCache(t, testConfig(), func(context.Context, string) (int, error) {
		close(loaderEntered)
		<-releaseLoader
		return 7, nil
	})

	result := make(chan error, 1)
	go func() {
		value, err := cache.GetOrLoad(context.Background(), "key")
		if err == nil && value != 7 {
			err = errors.New("unexpected value")
		}
		result <- err
	}()
	waitForSignal(t, loaderEntered)
	cache.Close()
	cache.Close()
	close(releaseLoader)
	require.NoError(t, <-result)

	_, err := cache.GetOrLoad(context.Background(), "key")
	require.ErrorIs(t, err, ErrClosed)
	require.ErrorIs(t, cache.Delete("key"), ErrClosed)
	require.ErrorIs(t, cache.Clear(), ErrClosed)
	require.Equal(t, "test", cache.Name())
	require.EqualValues(t, 1, cache.Stats().LoadSuccess)
}

func testConfig() Config {
	return Config{Name: "test", Capacity: 10, TTL: time.Minute, LoadTimeout: time.Second}
}

func newTestCache[K comparable, V any](t *testing.T, cfg Config, loader Loader[K, V]) *LoadingCache[K, V] {
	t.Helper()
	cache, err := NewLoadingCache(cfg, loader)
	require.NoError(t, err)
	t.Cleanup(cache.Close)
	return cache
}

func waitForSignal[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for test signal")
	}
}
