package localcache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
			if !errors.Is(err, tt.want) {
				t.Fatalf("New error = %v, want %v", err, tt.want)
			}
			if cache != nil {
				t.Fatal("New cache != nil, want nil")
			}
		})
	}
}

func TestCacheGetSetAndExpire(t *testing.T) {
	cache := newTestCache[int](t, Config[string]{Name: "test", Capacity: 10, TTL: 20 * time.Millisecond, KeyString: identityString}, nil)
	defer cache.Close()

	if _, ok, err := cache.Get("user-1"); err != nil || ok {
		t.Fatal("Get before Set = hit, want miss")
	}
	if ok, err := cache.Set("user-1", 7); err != nil || !ok {
		t.Fatalf("Set = (%v, %v), want (true, nil)", ok, err)
	}
	cache.client.Wait()
	version, ok, err := cache.Get("user-1")
	if err != nil || !ok || version != 7 {
		t.Fatalf("Get after Set = (%d, %v, %v), want (7, true, nil)", version, ok, err)
	}

	time.Sleep(50 * time.Millisecond)
	if _, ok, err := cache.Get("user-1"); err != nil || ok {
		t.Fatal("Get after TTL = hit, want miss")
	}
}

func TestCacheDeleteAndClear(t *testing.T) {
	cache := newTestCache[int](t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, nil)
	defer cache.Close()

	_, _ = cache.Set("a", 1)
	_, _ = cache.Set("b", 2)
	cache.client.Wait()

	if err := cache.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	cache.client.Wait()
	if _, ok, err := cache.Get("a"); err != nil || ok {
		t.Fatal("Get after Delete = hit, want miss")
	}
	if _, ok, err := cache.Get("b"); err != nil || !ok {
		t.Fatalf("Get unaffected key = (%v, %v), want hit", ok, err)
	}

	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	cache.client.Wait()
	if _, ok, err := cache.Get("b"); err != nil || ok {
		t.Fatal("Get after Clear = hit, want miss")
	}
}

func TestCacheGetOrLoadCoalescesConcurrentMisses(t *testing.T) {
	var loads atomic.Int64
	start := make(chan struct{})
	cache := newTestCache(t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, func(_ context.Context, key string) (int, error) {
		loads.Add(1)
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
	time.Sleep(20 * time.Millisecond)
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("GetOrLoad concurrent: %v", err)
		}
	}
	if got := loads.Load(); got != 1 {
		t.Fatalf("loads = %d, want 1", got)
	}
	stats := cache.Stats()
	if stats.Load != 1 {
		t.Fatalf("stats.Load = %d, want 1", stats.Load)
	}
	if stats.Shared == 0 {
		t.Fatal("stats.Shared = 0, want shared singleflight result")
	}
	if stats.Hit != 0 {
		t.Fatalf("stats.Hit = %d, want 0 for double-check-free initial miss wave", stats.Hit)
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
		if _, err := cache.GetOrLoad(context.Background(), "alice"); !errors.Is(err, loadErr) {
			t.Fatalf("GetOrLoad err = %v, want loadErr", err)
		}
	}
	if got := loads.Load(); got != 2 {
		t.Fatalf("loads = %d, want 2", got)
	}
	if got := cache.Stats().LoadError; got != 2 {
		t.Fatalf("LoadError = %d, want 2", got)
	}
}

func TestCacheCloneIsolatesLoaderCacheAndCaller(t *testing.T) {
	loaded := []int{1, 2}
	cache := newTestCache(t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, func(context.Context, string) ([]int, error) {
		return loaded, nil
	}, cloneInts)
	defer cache.Close()

	first, err := cache.GetOrLoad(context.Background(), "k")
	if err != nil {
		t.Fatalf("GetOrLoad first: %v", err)
	}
	first[0] = 99
	loaded[1] = 88
	cache.client.Wait()

	second, ok, err := cache.Get("k")
	if err != nil || !ok {
		t.Fatalf("Get cached = (%v, %v), want hit", ok, err)
	}
	if second[0] != 1 || second[1] != 2 {
		t.Fatalf("cached value = %#v, want [1 2]", second)
	}
	second[0] = 77
	third, ok, err := cache.Get("k")
	if err != nil || !ok || third[0] != 1 {
		t.Fatalf("third cached value = (%#v, %v, %v), want ([1 2], true, nil)", third, ok, err)
	}
}

func TestCacheCloseRejectsNewOperations(t *testing.T) {
	cache := newTestCache[int](t, Config[string]{Name: "test", Capacity: 10, TTL: time.Minute, KeyString: identityString}, nil)
	cache.Close()

	if _, ok, err := cache.Get("k"); !errors.Is(err, ErrClosed) || ok {
		t.Fatalf("Get after Close = (%v, %v), want (false, ErrClosed)", ok, err)
	}
	if _, err := cache.GetOrLoad(context.Background(), "k"); !errors.Is(err, ErrClosed) {
		t.Fatalf("GetOrLoad after Close err = %v, want ErrClosed", err)
	}
	if ok, err := cache.Set("k", 1); !errors.Is(err, ErrClosed) || ok {
		t.Fatalf("Set after Close = (%v, %v), want (false, ErrClosed)", ok, err)
	}
	if err := cache.Delete("k"); !errors.Is(err, ErrClosed) {
		t.Fatalf("Delete after Close err = %v, want ErrClosed", err)
	}
	if err := cache.Clear(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Clear after Close err = %v, want ErrClosed", err)
	}
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
	if err != nil {
		t.Fatalf("detached loader err = %v, want nil", err)
	}
	if result.(int) != 7 {
		t.Fatalf("detached loader result = %v, want 7", result)
	}
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
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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
