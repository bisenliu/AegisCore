package localcache

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"golang.org/x/sync/singleflight"
)

type flightEntry struct {
	group singleflight.Group
	refs  int
}

// LoadingCache 是 bounded、固定 TTL 的本地 loading cache。
// 可变 value 的复制语义由调用方负责。
type LoadingCache[K comparable, V any] struct {
	name        string
	capacity    uint64
	loadTimeout time.Duration
	loader      Loader[K, V]
	client      *ttlcache.Cache[K, V]

	lifecycleMu sync.RWMutex
	closed      bool
	unsubscribe func()
	cleanerStop chan struct{}
	cleanerDone chan struct{}

	flightsMu sync.Mutex
	flights   map[K]*flightEntry

	hit         atomic.Uint64
	miss        atomic.Uint64
	loadSuccess atomic.Uint64
	loadError   atomic.Uint64
	evicted     atomic.Uint64
}

// NewLoadingCache 创建并启动本地 loading cache。
func NewLoadingCache[K comparable, V any](cfg Config, loader Loader[K, V]) (*LoadingCache[K, V], error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if cfg.Capacity == 0 {
		return nil, ErrCapacityRequired
	}
	if cfg.TTL <= 0 {
		return nil, ErrTTLRequired
	}
	if cfg.LoadTimeout <= 0 {
		return nil, ErrLoadTimeoutRequired
	}
	if loader == nil {
		return nil, ErrLoaderRequired
	}

	cache := &LoadingCache[K, V]{
		name:        name,
		capacity:    cfg.Capacity,
		loadTimeout: cfg.LoadTimeout,
		loader:      loader,
		flights:     make(map[K]*flightEntry),
		cleanerStop: make(chan struct{}),
		cleanerDone: make(chan struct{}),
	}
	cache.client = ttlcache.New[K, V](
		ttlcache.WithTTL[K, V](cfg.TTL),
		ttlcache.WithCapacity[K, V](cfg.Capacity),
		ttlcache.WithDisableTouchOnHit[K, V](),
	)
	cache.unsubscribe = cache.client.OnEviction(func(_ context.Context, reason ttlcache.EvictionReason, _ *ttlcache.Item[K, V]) {
		if reason == ttlcache.EvictionReasonExpired || reason == ttlcache.EvictionReasonCapacityReached {
			cache.evicted.Add(1)
		}
	})
	go cache.cleanExpired(cfg.TTL)
	return cache, nil
}

// Name 返回缓存实例的稳定名称。
func (c *LoadingCache[K, V]) Name() string {
	return c.name
}

// GetOrLoad 返回未过期值；miss 时合并同 key 并发回源。
func (c *LoadingCache[K, V]) GetOrLoad(ctx context.Context, key K) (V, error) {
	var zero V
	if ctx == nil {
		ctx = context.Background()
	}
	if value, ok, err := c.lookup(key); err != nil {
		return zero, err
	} else if ok {
		c.hit.Add(1)
		return value, nil
	}
	c.miss.Add(1)

	entry := c.acquireFlight(key)
	result := entry.group.DoChan("", func() (any, error) {
		if value, ok, err := c.lookup(key); err != nil {
			return zero, err
		} else if ok {
			return value, nil
		}

		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.loadTimeout)
		defer cancel()
		loaded, err := c.loader(loadCtx, key)
		if err != nil {
			c.loadError.Add(1)
			return zero, err
		}
		c.loadSuccess.Add(1)

		c.lifecycleMu.RLock()
		if !c.closed {
			c.client.Set(key, loaded, ttlcache.DefaultTTL)
		}
		c.lifecycleMu.RUnlock()
		return loaded, nil
	})

	select {
	case <-ctx.Done():
		go func() {
			<-result
			c.releaseFlight(key, entry)
		}()
		return zero, ctx.Err()
	case loaded := <-result:
		c.releaseFlight(key, entry)
		if loaded.Err != nil {
			return zero, loaded.Err
		}
		return loaded.Val.(V), nil
	}
}

// Delete 同步删除指定 key。
func (c *LoadingCache[K, V]) Delete(key K) error {
	c.lifecycleMu.RLock()
	defer c.lifecycleMu.RUnlock()
	if c.closed {
		return ErrClosed
	}
	c.client.Delete(key)
	return nil
}

// Clear 同步删除全部 item。
func (c *LoadingCache[K, V]) Clear() error {
	c.lifecycleMu.RLock()
	defer c.lifecycleMu.RUnlock()
	if c.closed {
		return ErrClosed
	}
	c.client.DeleteAll()
	return nil
}

// Stats 返回当前累计统计快照。自动驱逐 callback 可能最终可见。
func (c *LoadingCache[K, V]) Stats() Stats {
	return Stats{
		Hit:         c.hit.Load(),
		Miss:        c.miss.Load(),
		LoadSuccess: c.loadSuccess.Load(),
		LoadError:   c.loadError.Load(),
		Evicted:     c.evicted.Load(),
		Capacity:    c.capacity,
	}
}

// Close 幂等停止后台清理，并阻止新的 cache 操作。
func (c *LoadingCache[K, V]) Close() {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.cleanerStop)
	<-c.cleanerDone
	if c.unsubscribe != nil {
		c.unsubscribe()
		c.unsubscribe = nil
	}
}

func (c *LoadingCache[K, V]) cleanExpired(interval time.Duration) {
	defer close(c.cleanerDone)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.client.DeleteExpired()
		case <-c.cleanerStop:
			return
		}
	}
}

func (c *LoadingCache[K, V]) lookup(key K) (V, bool, error) {
	var zero V
	c.lifecycleMu.RLock()
	defer c.lifecycleMu.RUnlock()
	if c.closed {
		return zero, false, ErrClosed
	}
	item := c.client.Get(key)
	if item == nil {
		return zero, false, nil
	}
	return item.Value(), true, nil
}

func (c *LoadingCache[K, V]) acquireFlight(key K) *flightEntry {
	c.flightsMu.Lock()
	defer c.flightsMu.Unlock()
	entry := c.flights[key]
	if entry == nil {
		entry = &flightEntry{}
		c.flights[key] = entry
	}
	entry.refs++
	return entry
}

func (c *LoadingCache[K, V]) releaseFlight(key K, entry *flightEntry) {
	c.flightsMu.Lock()
	defer c.flightsMu.Unlock()
	entry.refs--
	if entry.refs == 0 && c.flights[key] == entry {
		delete(c.flights, key)
	}
}
