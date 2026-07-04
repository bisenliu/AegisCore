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

func TestNewValidatesConfig(t *testing.T) {
	base := Config[string]{Name: "test", Capacity: 10, TTL: time.Second, KeyString: func(key string) string { return key }}
	loader := func(context.Context, string) (int, error) { return 1, nil }
	tests := []struct {
		name string
		cfg  Config[string]
		load Loader[string, int]
		want error
	}{
		{name: "missing name", cfg: Config[string]{Capacity: 10, TTL: time.Second, KeyString: base.KeyString}, load: loader, want: ErrNameRequired},
		{name: "missing capacity", cfg: Config[string]{Name: "test", TTL: time.Second, KeyString: base.KeyString}, load: loader, want: ErrCapacityRequired},
		{name: "missing ttl", cfg: Config[string]{Name: "test", Capacity: 10, KeyString: base.KeyString}, load: loader, want: ErrTTLRequired},
		{name: "missing key", cfg: Config[string]{Name: "test", Capacity: 10, TTL: time.Second}, load: loader, want: ErrKeyStringRequired},
		{name: "missing loader", cfg: base, load: nil, want: ErrLoaderRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := New[string, int](tt.cfg, tt.load, nil)
			require.ErrorIs(t, err, tt.want)
			require.Nil(t, cache)
		})
	}
}

func TestCacheGetSetAndExpire(t *testing.T) {
	cache := newTestCache[int](t, Config[string]{Name: "test", Capacity: 10, TTL: 20 * time.Millisecond, KeyString: identityString}, nil)
	defer cache.Close()

	_, ok, err := cache.Get("user-1")
	require.NoError(t, err)
	require.False(t, ok, "Get before Set = hit, want miss")
	ok, err = cache.Set("user-1", 7)
	require.NoError(t, err)
	require.True(t, ok)
	cache.client.Wait()
	version, ok, err := cache.Get("user-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 7, version)

	require.Eventually(t, func() bool {
		_, ok, err = cache.Get("user-1")
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond, "Get after TTL = hit, want miss")
}

func TestCacheDeleteAndClear(t *testing.T) {
	cache := newTestCache[int](t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, nil)
	defer cache.Close()

	_, _ = cache.Set("a", 1)
	_, _ = cache.Set("b", 2)
	cache.client.Wait()

	require.NoError(t, cache.Delete("a"), "Delete")
	cache.client.Wait()
	_, ok, err := cache.Get("a")
	require.NoError(t, err)
	require.False(t, ok, "Get after Delete = hit, want miss")
	_, ok, err = cache.Get("b")
	require.NoError(t, err)
	require.True(t, ok, "Get unaffected key = miss, want hit")

	require.NoError(t, cache.Clear(), "Clear")
	cache.client.Wait()
	_, ok, err = cache.Get("b")
	require.NoError(t, err)
	require.False(t, ok, "Get after Clear = hit, want miss")
}

func TestCacheGetOrLoadCoalescesConcurrentMisses(t *testing.T) {
	var loads atomic.Int64
	var loaderStarted sync.Once
	callersReady := make(chan struct{}, 20)
	callersReleased := make(chan struct{})
	loaderEntered := make(chan struct{})
	start := make(chan struct{})
	cache := newTestCache(t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, func(_ context.Context, key string) (int, error) {
		loads.Add(1)
		loaderStarted.Do(func() { close(loaderEntered) })
		<-start
		return len(key), nil
	})
	defer cache.Close()

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			callersReady <- struct{}{}
			<-callersReleased
			value, err := cache.GetOrLoad(context.Background(), "alice")
			if err != nil {
				errs <- err
				return
			}
			if value != 5 {
				errs <- errors.New("unexpected value")
			}
		}()
	}
	waitForCacheTestSignals(t, callersReady, goroutines)
	close(callersReleased)
	waitForCacheTestSignal(t, loaderEntered)
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err, "GetOrLoad concurrent")
	}
	require.EqualValues(t, 1, loads.Load(), "loads")
	stats := cache.Stats()
	require.EqualValues(t, 1, stats.Load)
	require.NotZero(t, stats.Shared, "stats.Shared = 0, want shared singleflight result")
	require.Zero(t, stats.Hit, "stats.Hit want 0 for double-check-free initial miss wave")
}

func waitForCacheTestSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cache test signal")
	}
}

func waitForCacheTestSignals(t *testing.T, ch <-chan struct{}, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		waitForCacheTestSignal(t, ch)
	}
}

func TestCacheGetOrLoadDoesNotCacheErrors(t *testing.T) {
	loadErr := errors.New("load failed")
	var loads atomic.Int64
	cache := newTestCache(t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, func(context.Context, string) (int, error) {
		loads.Add(1)
		return 0, loadErr
	})
	defer cache.Close()

	for i := 0; i < 2; i++ {
		_, err := cache.GetOrLoad(context.Background(), "alice")
		require.ErrorIs(t, err, loadErr)
	}
	require.EqualValues(t, 2, loads.Load(), "loads")
	require.EqualValues(t, 2, cache.Stats().LoadError, "LoadError")
}

func TestCacheCloneIsolatesLoaderCacheAndCaller(t *testing.T) {
	loaded := []int{1, 2}
	cache := newTestCache(t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, func(context.Context, string) ([]int, error) {
		return loaded, nil
	}, cloneInts)
	defer cache.Close()

	first, err := cache.GetOrLoad(context.Background(), "k")
	require.NoError(t, err, "GetOrLoad first")
	first[0] = 99
	loaded[1] = 88
	cache.client.Wait()

	second, ok, err := cache.Get("k")
	require.NoError(t, err)
	require.True(t, ok, "Get cached = miss, want hit")
	require.Equal(t, []int{1, 2}, second)
	second[0] = 77
	third, ok, err := cache.Get("k")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []int{1, 2}, third)
}

func TestCacheCloseRejectsNewOperations(t *testing.T) {
	cache := newTestCache[int](t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, nil)
	cache.Close()

	_, ok, err := cache.Get("k")
	require.ErrorIs(t, err, ErrClosed)
	require.False(t, ok)
	_, err = cache.GetOrLoad(context.Background(), "k")
	require.ErrorIs(t, err, ErrClosed)
	ok, err = cache.Set("k", 1)
	require.ErrorIs(t, err, ErrClosed)
	require.False(t, ok)
	require.ErrorIs(t, cache.Delete("k"), ErrClosed)
	require.ErrorIs(t, cache.Clear(), ErrClosed)
	cache.Close()
}

func TestCacheLoadTimeoutDetachesRequestCancellation(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	cache := newTestCache(t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, LoadTimeout: time.Second, KeyString: identityString}, func(ctx context.Context, _ string) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			return 7, nil
		}
	})
	defer cache.Close()

	result, err, _ := cache.group.Do("detached", func() (any, error) {
		loadCtx, cancel := cache.loadContext(requestCtx)
		defer cancel()
		return cache.loader(loadCtx, "k")
	})
	require.NoError(t, err, "detached loader")
	require.Equal(t, 7, result)
}

func newTestCache[V any](t *testing.T, cfg Config[string], loader Loader[string, V], clone ...CloneFunc[V]) *Cache[string, V] {
	t.Helper()
	if loader == nil {
		loader = func(context.Context, string) (V, error) {
			var zero V
			return zero, nil
		}
	}
	var cloneFunc CloneFunc[V]
	if len(clone) > 0 {
		cloneFunc = clone[0]
	}
	cache, err := New[string, V](cfg, loader, cloneFunc)
	require.NoError(t, err, "New")
	return cache
}

func identityString(key string) string {
	return key
}

func cloneInts(values []int) []int {
	if values == nil {
		return nil
	}
	cloned := make([]int, len(values))
	copy(cloned, values)
	return cloned
}
