package localcache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"golang.org/x/sync/singleflight"
)

var errLoadInvalidated = errors.New("localcache internal load invalidated")

// LoadingCache 是 bounded、固定 TTL 的本地 loading cache。
// 可变 value 的复制语义由调用方负责。
type LoadingCache[V any] struct {
	name        string
	capacity    uint64
	loadTimeout time.Duration
	loader      Loader[V]
	client      *ttlcache.Cache[string, V]
	loads       singleflight.Group

	publishMu sync.Mutex
	revision  uint64

	hit               atomic.Uint64
	miss              atomic.Uint64
	loadSuccess       atomic.Uint64
	loadError         atomic.Uint64
	capacityEvictions atomic.Uint64
}

// NewLoadingCache 创建本地 loading cache。
func NewLoadingCache[V any](cfg Config, loader Loader[V]) (*LoadingCache[V], error) {
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

	cache := &LoadingCache[V]{
		name:        name,
		capacity:    cfg.Capacity,
		loadTimeout: cfg.LoadTimeout,
		loader:      loader,
	}
	cache.client = ttlcache.New[string, V](
		ttlcache.WithTTL[string, V](cfg.TTL),
		ttlcache.WithCapacity[string, V](cfg.Capacity),
		ttlcache.WithDisableTouchOnHit[string, V](),
	)
	cache.client.OnEviction(func(_ context.Context, reason ttlcache.EvictionReason, _ *ttlcache.Item[string, V]) {
		if reason == ttlcache.EvictionReasonCapacityReached {
			cache.capacityEvictions.Add(1)
		}
	})
	return cache, nil
}

// Name 返回缓存实例的稳定名称。
func (c *LoadingCache[V]) Name() string {
	return c.name
}

// Get 返回未过期值；miss 时合并同 key 并发回源。
func (c *LoadingCache[V]) Get(ctx context.Context, key string) (V, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if value, ok := c.lookup(key); ok {
		c.hit.Add(1)
		return value, nil
	}
	c.miss.Add(1)

	for attempt := 0; attempt < 2; attempt++ {
		value, err := c.load(ctx, key)
		if !errors.Is(err, errLoadInvalidated) {
			return value, err
		}
	}

	var zero V
	return zero, ErrInvalidated
}

// Invalidate 同步失效指定 key，并抑制此前开始的 loader。
func (c *LoadingCache[V]) Invalidate(key string) {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	c.revision++
	c.client.Delete(key)
}

// InvalidateAll 同步失效全部 item，并抑制此前开始的 loader。
func (c *LoadingCache[V]) InvalidateAll() {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	c.revision++
	c.client.DeleteAll()
}

// Stats 返回当前累计统计快照。
func (c *LoadingCache[V]) Stats() Stats {
	return Stats{
		Hit:               c.hit.Load(),
		Miss:              c.miss.Load(),
		LoadSuccess:       c.loadSuccess.Load(),
		LoadError:         c.loadError.Load(),
		CapacityEvictions: c.capacityEvictions.Load(),
		Capacity:          c.capacity,
	}
}

func (c *LoadingCache[V]) load(ctx context.Context, key string) (V, error) {
	result := c.loads.DoChan(key, func() (any, error) {
		if value, ok := c.lookup(key); ok {
			return value, nil
		}

		c.publishMu.Lock()
		startRevision := c.revision
		c.publishMu.Unlock()

		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.loadTimeout)
		defer cancel()
		loaded, err := c.loader(loadCtx, key)
		if err != nil {
			c.loadError.Add(1)
			return nil, err
		}
		c.loadSuccess.Add(1)

		c.publishMu.Lock()
		defer c.publishMu.Unlock()
		if c.revision != startRevision {
			return nil, errLoadInvalidated
		}
		c.client.DeleteExpired()
		c.client.Set(key, loaded, ttlcache.DefaultTTL)
		return loaded, nil
	})

	var zero V
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case loaded := <-result:
		if loaded.Err != nil {
			return zero, loaded.Err
		}
		return loaded.Val.(V), nil
	}
}

func (c *LoadingCache[V]) lookup(key string) (V, bool) {
	var zero V
	item := c.client.Get(key)
	if item == nil {
		return zero, false
	}
	return item.Value(), true
}
