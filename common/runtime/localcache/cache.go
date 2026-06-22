package localcache

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"golang.org/x/sync/singleflight"
)

const defaultBufferItems int64 = 64

var (
	// ErrNameRequired 表示本地缓存缺少稳定实例名。
	ErrNameRequired = errors.New("localcache name is required")
	// ErrCapacityRequired 表示本地缓存容量预算必须为正数。
	ErrCapacityRequired = errors.New("localcache capacity must be positive")
	// ErrTTLRequired 表示本地缓存 TTL 必须为正数。
	ErrTTLRequired = errors.New("localcache ttl must be positive")
	// ErrKeyStringRequired 表示本地缓存缺少 key 编码函数。
	ErrKeyStringRequired = errors.New("localcache key string function is required")
	// ErrLoaderRequired 表示 loading cache 缺少回源函数。
	ErrLoaderRequired = errors.New("localcache loader is required")
	// ErrClosed 表示缓存实例已关闭并拒绝新操作。
	ErrClosed = errors.New("localcache is closed")
)

// Loader 定义缓存 miss 后的回源函数。
type Loader[K comparable, V any] func(context.Context, K) (V, error)

// CloneFunc 返回 value 副本，用于隔离 loader、cache 和调用方持有的可变对象。
type CloneFunc[V any] func(V) V

// Config 描述一个本地缓存实例。
type Config[K comparable] struct {
	Name        string
	Capacity    int64
	TTL         time.Duration
	LoadTimeout time.Duration
	KeyString   func(K) string
	NumCounters int64
	BufferItems int64
}

// Stats 是 localcache 暴露给 metrics collector 的稳定统计快照。
type Stats struct {
	Hit            uint64
	Miss           uint64
	Load           uint64
	LoadError      uint64
	Shared         uint64
	DoubleCheckHit uint64
	SetDropped     uint64
	Rejected       uint64
	Evicted        uint64
	Capacity       int64
}

// StatsSource 定义可导出 localcache 统计快照的类型。
type StatsSource interface {
	Name() string
	Stats() Stats
}

// Cache 是基于 Ristretto 的 bounded TTL loading cache。
type Cache[K comparable, V any] struct {
	name        string
	capacity    int64
	client      *ristretto.Cache[string, V]
	group       singleflight.Group
	ttl         time.Duration
	loadTimeout time.Duration
	keyString   func(K) string
	loader      Loader[K, V]
	clone       CloneFunc[V]
	closed      atomic.Bool

	hit            atomic.Uint64
	miss           atomic.Uint64
	load           atomic.Uint64
	loadError      atomic.Uint64
	shared         atomic.Uint64
	doubleCheckHit atomic.Uint64
	setDropped     atomic.Uint64
	rejected       atomic.Uint64
	evicted        atomic.Uint64
}

// New 创建本地 loading cache。
func New[K comparable, V any](cfg Config[K], loader Loader[K, V], clone CloneFunc[V]) (*Cache[K, V], error) {
	if cfg.Name == "" {
		return nil, ErrNameRequired
	}
	if cfg.Capacity <= 0 {
		return nil, ErrCapacityRequired
	}
	if cfg.TTL <= 0 {
		return nil, ErrTTLRequired
	}
	if cfg.KeyString == nil {
		return nil, ErrKeyStringRequired
	}
	if loader == nil {
		return nil, ErrLoaderRequired
	}
	if clone == nil {
		clone = func(v V) V { return v }
	}

	numCounters := cfg.NumCounters
	if numCounters <= 0 {
		numCounters = cfg.Capacity * 10
	}
	bufferItems := cfg.BufferItems
	if bufferItems <= 0 {
		bufferItems = defaultBufferItems
	}

	cache := &Cache[K, V]{
		name:        cfg.Name,
		capacity:    cfg.Capacity,
		ttl:         cfg.TTL,
		loadTimeout: cfg.LoadTimeout,
		keyString:   cfg.KeyString,
		loader:      loader,
		clone:       clone,
	}

	client, err := ristretto.NewCache(&ristretto.Config[string, V]{
		NumCounters:        numCounters,
		MaxCost:            cfg.Capacity,
		BufferItems:        bufferItems,
		IgnoreInternalCost: true,
		Metrics:            false,
		OnReject:           func(*ristretto.Item[V]) { cache.rejected.Add(1) },
		OnEvict:            func(*ristretto.Item[V]) { cache.evicted.Add(1) },
	})
	if err != nil {
		return nil, fmt.Errorf("init localcache %q: %w", cfg.Name, err)
	}
	cache.client = client
	return cache, nil
}

// Name 返回缓存实例名称。
func (c *Cache[K, V]) Name() string {
	return c.name
}

// Get 读取缓存，并记录业务 hit/miss。
func (c *Cache[K, V]) Get(key K) (V, bool, error) {
	var zero V
	if c.closed.Load() {
		return zero, false, ErrClosed
	}
	value, ok := c.lookup(c.keyString(key))
	if ok {
		c.hit.Add(1)
		return c.clone(value), true, nil
	}
	c.miss.Add(1)
	return zero, false, nil
}

// GetOrLoad 读取缓存；miss 时通过 singleflight 合并同 key 回源。
func (c *Cache[K, V]) GetOrLoad(ctx context.Context, key K) (V, error) {
	var zero V
	if c.closed.Load() {
		return zero, ErrClosed
	}

	cacheKey := c.keyString(key)
	if value, ok := c.lookup(cacheKey); ok {
		c.hit.Add(1)
		return c.clone(value), nil
	}
	c.miss.Add(1)

	ch := c.group.DoChan(cacheKey, func() (any, error) {
		if c.closed.Load() {
			return zero, ErrClosed
		}
		if value, ok := c.lookup(cacheKey); ok {
			c.doubleCheckHit.Add(1)
			return c.clone(value), nil
		}

		c.load.Add(1)
		loadCtx, cancel := c.loadContext(ctx)
		defer cancel()

		loaded, err := c.loader(loadCtx, key)
		if err != nil {
			c.loadError.Add(1)
			return zero, err
		}

		cached := c.clone(loaded)
		if ok := c.client.SetWithTTL(cacheKey, cached, 1, c.ttl); !ok {
			c.setDropped.Add(1)
		} else {
			c.client.Wait()
		}
		return loaded, nil
	})

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case result := <-ch:
		if result.Shared {
			c.shared.Add(1)
		}
		if result.Err != nil {
			return zero, result.Err
		}
		return result.Val.(V), nil
	}
}

// Set 写入缓存，适合预热或业务主动刷新。
func (c *Cache[K, V]) Set(key K, value V) (bool, error) {
	if c.closed.Load() {
		return false, ErrClosed
	}
	cached := c.clone(value)
	ok := c.client.SetWithTTL(c.keyString(key), cached, 1, c.ttl)
	if !ok {
		c.setDropped.Add(1)
	} else {
		c.client.Wait()
	}
	return ok, nil
}

// Delete 删除单个缓存项。
func (c *Cache[K, V]) Delete(key K) error {
	if c.closed.Load() {
		return ErrClosed
	}
	c.client.Del(c.keyString(key))
	c.client.Wait()
	return nil
}

// Clear 清空缓存。
func (c *Cache[K, V]) Clear() error {
	if c.closed.Load() {
		return ErrClosed
	}
	c.client.Clear()
	c.client.Wait()
	return nil
}

// Stats 返回当前统计快照。
func (c *Cache[K, V]) Stats() Stats {
	return Stats{
		Hit:            c.hit.Load(),
		Miss:           c.miss.Load(),
		Load:           c.load.Load(),
		LoadError:      c.loadError.Load(),
		Shared:         c.shared.Load(),
		DoubleCheckHit: c.doubleCheckHit.Load(),
		SetDropped:     c.setDropped.Load(),
		Rejected:       c.rejected.Load(),
		Evicted:        c.evicted.Load(),
		Capacity:       c.capacity,
	}
}

// Close 关闭缓存后台资源。
func (c *Cache[K, V]) Close() {
	if c.closed.CompareAndSwap(false, true) {
		c.client.Close()
	}
}

func (c *Cache[K, V]) lookup(cacheKey string) (V, bool) {
	return c.client.Get(cacheKey)
}

// loadContext 会刻意解除请求 ctx 的取消信号，避免 singleflight leader
// 因客户端断开导致所有 follower 一起收到 context.Canceled；LoadTimeout
// 用于给回源路径重新建立独立上限。
func (c *Cache[K, V]) loadContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.loadTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), c.loadTimeout)
}
