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

// maxLoadAttempts 是本地缓存回源加载在遭遇并发失效时的最大尝试次数（初始 1 次 + 重试 1 次）。
const maxLoadAttempts = 2

// LoadingCache 是具有容量上限（防内存撑爆）、固定过期时间（TTL）并支持并发自动加载的本地内存缓存。
// 缓存中存储的对象若会被外部修改，调用方应自行深拷贝，避免污染共享缓存中的数据。
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
// 详细的配置说明、回源加载、并发合并、主动失效、指标统计和数据防污染机制，请参阅 package localcache 文档与 ExampleLoadingCache。
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

	// 失效与回源竞争时允许重试一次；持续失效则尽快返回，避免请求无限占用执行资源。
	for attempt := 0; attempt < maxLoadAttempts; attempt++ {
		value, err := c.load(ctx, key)
		if !errors.Is(err, errLoadInvalidated) {
			return value, err
		}
	}

	var zero V
	return zero, ErrInvalidated
}

// lookup 快速查看本地内存缓存中是否已经存有该数据；如果未命中则直接返回不存在，不在此处触发耗时的回源加载。
func (c *LoadingCache[V]) lookup(key string) (V, bool) {
	var zero V
	item := c.client.Get(key)
	if item == nil {
		return zero, false
	}
	return item.Value(), true
}
