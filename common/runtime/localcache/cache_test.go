package localcache

import (
	"context"
	"errors"
	"runtime"
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
		load Loader[int]
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
			cache, err := NewLoadingCache(tt.cfg, tt.load)
			require.ErrorIs(t, err, tt.want)
			require.Nil(t, cache)
		})
	}
}

func TestLoadingCacheUsesFixedTTLWithoutTouchOnHit(t *testing.T) {
	var loads atomic.Uint64
	cache := newTestCache(t, Config{Name: "test", Capacity: 10, TTL: 100 * time.Millisecond, LoadTimeout: time.Second}, func(context.Context, string) (int, error) {
		return int(loads.Add(1)), nil
	})

	first, err := cache.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, 1, first)
	time.Sleep(60 * time.Millisecond)
	value, err := cache.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, 1, value)
	time.Sleep(60 * time.Millisecond)
	value, err = cache.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, 2, value, "读取不应延长首次写入的 TTL")
	require.Equal(t, Stats{Hit: 1, Miss: 2, LoadSuccess: 2, Capacity: 10}, cache.Stats())
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

	_, err := cache.Get(context.Background(), "a")
	require.NoError(t, err)
	_, err = cache.Get(context.Background(), "b")
	require.NoError(t, err)
	require.EqualValues(t, 1, cache.Stats().CapacityEvictions, "容量驱逐应在 Get 返回前同步可见")
	value, err := cache.Get(context.Background(), "a")
	require.NoError(t, err)
	require.Equal(t, 2, value, "最早的 item 应因容量达到上限被移除")
	require.EqualValues(t, 2, cache.Stats().CapacityEvictions)
}

func TestLoadingCacheCachesNilInterfaceValue(t *testing.T) {
	var loads atomic.Uint64
	cache := newTestCache[any](t, testConfig(), func(context.Context, string) (any, error) {
		loads.Add(1)
		return nil, nil
	})

	first, err := cache.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Nil(t, first)

	second, err := cache.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Nil(t, second)
	require.EqualValues(t, 1, loads.Load())
	require.Equal(t, Stats{Hit: 1, Miss: 1, LoadSuccess: 1, Capacity: 10}, cache.Stats())
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
			value, err := cache.Get(context.Background(), "alice")
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
			_, err := cache.Get(context.Background(), key)
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
		_, err := cache.Get(context.Background(), "key")
		require.ErrorIs(t, err, loadErr)
	}
	require.EqualValues(t, 2, loads.Load())
	require.Equal(t, Stats{Miss: 2, LoadError: 2, Capacity: 10}, cache.Stats())
}

func TestLoadingCacheCallerCancellationDoesNotCancelSharedLoader(t *testing.T) {
	type contextKey struct{}
	loaderEntered := make(chan struct{})
	releaseLoader := make(chan struct{})
	cache := newTestCache(t, testConfig(), func(ctx context.Context, _ string) (string, error) {
		close(loaderEntered)
		if ctx.Value(contextKey{}) != "request-value" {
			return "", errors.New("loader context value was not preserved")
		}
		<-releaseLoader
		return "loaded", ctx.Err()
	})

	leaderCtx, cancelLeader := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "request-value"))
	leaderResult := make(chan error, 1)
	go func() {
		_, err := cache.Get(leaderCtx, "key")
		leaderResult <- err
	}()
	waitForSignal(t, loaderEntered)

	followerResult := make(chan error, 1)
	go func() {
		value, err := cache.Get(context.Background(), "key")
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

func TestLoadingCacheLoadTimeout(t *testing.T) {
	cache := newTestCache(t, Config{Name: "test", Capacity: 10, TTL: time.Minute, LoadTimeout: 20 * time.Millisecond}, func(ctx context.Context, _ string) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})

	_, err := cache.Get(context.Background(), "key")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.EqualValues(t, 1, cache.Stats().LoadError)
}

func TestLoadingCacheInvalidationSuppressesInflightValue(t *testing.T) {
	for _, tt := range []struct {
		name       string
		invalidate func(*LoadingCache[int])
	}{
		{name: "single key", invalidate: func(cache *LoadingCache[int]) { cache.Invalidate("key") }},
		{name: "all keys", invalidate: func(cache *LoadingCache[int]) { cache.InvalidateAll() }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var source atomic.Int64
			source.Store(1)
			var calls atomic.Uint64
			started := make(chan uint64, 2)
			releases := []chan struct{}{make(chan struct{}), make(chan struct{})}
			cache := newTestCache(t, testConfig(), func(context.Context, string) (int, error) {
				call := calls.Add(1)
				value := int(source.Load())
				started <- call
				<-releases[call-1]
				return value, nil
			})

			result := make(chan struct {
				value int
				err   error
			}, 1)
			go func() {
				value, err := cache.Get(context.Background(), "key")
				result <- struct {
					value int
					err   error
				}{value: value, err: err}
			}()

			require.EqualValues(t, 1, waitForSignal(t, started))
			source.Store(2)
			tt.invalidate(cache)
			close(releases[0])
			require.EqualValues(t, 2, waitForSignal(t, started))
			close(releases[1])

			got := waitForSignal(t, result)
			require.NoError(t, got.err)
			require.Equal(t, 2, got.value)
			cached, err := cache.Get(context.Background(), "key")
			require.NoError(t, err)
			require.Equal(t, 2, cached)
			require.EqualValues(t, 2, calls.Load())
			require.Equal(t, Stats{Hit: 1, Miss: 1, LoadSuccess: 2, Capacity: 10}, cache.Stats())
		})
	}
}

func TestLoadingCacheReturnsErrInvalidatedAfterSecondConflict(t *testing.T) {
	var source atomic.Int64
	source.Store(1)
	var calls atomic.Uint64
	started := make(chan uint64, 2)
	releases := []chan struct{}{make(chan struct{}), make(chan struct{})}
	cache := newTestCache(t, testConfig(), func(context.Context, string) (int, error) {
		call := calls.Add(1)
		value := int(source.Load())
		if call <= 2 {
			started <- call
			<-releases[call-1]
		}
		return value, nil
	})

	result := make(chan error, 1)
	go func() {
		_, err := cache.Get(context.Background(), "key")
		result <- err
	}()
	require.EqualValues(t, 1, waitForSignal(t, started))
	source.Store(2)
	cache.Invalidate("key")
	close(releases[0])
	require.EqualValues(t, 2, waitForSignal(t, started))
	source.Store(3)
	cache.Invalidate("key")
	close(releases[1])
	require.ErrorIs(t, waitForSignal(t, result), ErrInvalidated)

	value, err := cache.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, 3, value)
	require.EqualValues(t, 3, calls.Load())
	require.Equal(t, Stats{Miss: 2, LoadSuccess: 3, Capacity: 10}, cache.Stats())
}

func TestLoadingCacheCanceledCallersDoNotWaitInDrainGoroutines(t *testing.T) {
	baseline := runtime.NumGoroutine()
	loaderEntered := make(chan struct{})
	releaseLoader := make(chan struct{})
	defer close(releaseLoader)
	cache := newTestCache(t, testConfig(), func(context.Context, string) (int, error) {
		select {
		case <-loaderEntered:
		default:
			close(loaderEntered)
		}
		<-releaseLoader
		return 1, nil
	})

	const callers = 100
	contexts := make([]context.CancelFunc, 0, callers)
	var wg sync.WaitGroup
	for range callers {
		ctx, cancel := context.WithCancel(context.Background())
		contexts = append(contexts, cancel)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.Get(ctx, "key")
		}()
	}
	waitForSignal(t, loaderEntered)
	require.Eventually(t, func() bool { return cache.Stats().Miss == callers }, time.Second, time.Millisecond)
	for _, cancel := range contexts {
		cancel()
	}
	wg.Wait()

	require.Eventually(t, func() bool {
		runtime.GC()
		return runtime.NumGoroutine() <= baseline+10
	}, time.Second, 10*time.Millisecond)
}

func TestLoadingCacheExpiryAndInvalidationAreNotCapacityEvictions(t *testing.T) {
	var loads atomic.Uint64
	cache := newTestCache(t, Config{Name: "test", Capacity: 10, TTL: 20 * time.Millisecond, LoadTimeout: time.Second}, func(context.Context, string) (uint64, error) {
		return loads.Add(1), nil
	})

	_, err := cache.Get(context.Background(), "key")
	require.NoError(t, err)
	time.Sleep(30 * time.Millisecond)
	_, err = cache.Get(context.Background(), "key")
	require.NoError(t, err)
	cache.Invalidate("key")
	_, err = cache.Get(context.Background(), "key")
	require.NoError(t, err)
	cache.InvalidateAll()
	_, err = cache.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Zero(t, cache.Stats().CapacityEvictions)
}

func testConfig() Config {
	return Config{Name: "test", Capacity: 10, TTL: time.Minute, LoadTimeout: time.Second}
}

func newTestCache[V any](t *testing.T, cfg Config, loader Loader[V]) *LoadingCache[V] {
	t.Helper()
	cache, err := NewLoadingCache(cfg, loader)
	require.NoError(t, err)
	return cache
}

func waitForSignal[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for test signal")
		var zero T
		return zero
	}
}
