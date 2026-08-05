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

// NewLoadingCache 创建一个有容量上限、固定 TTL、支持并发回源合并的进程内缓存。
// 完整的配置、回源、取消、强失效、统计和值所有权契约参见 package localcache 文档与 ExampleLoadingCache。
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

func (c *LoadingCache[V]) lookup(key string) (V, bool) {
	var zero V
	item := c.client.Get(key)
	if item == nil {
		return zero, false
	}
	return item.Value(), true
}
