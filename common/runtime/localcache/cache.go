package localcache

import (
	"sync"
	"time"
)

// Cache 是进程内短 TTL 缓存，用于减少同一实例内重复读取远端缓存或存储的开销。
//
// 用途：
//   - 缓存可以按 key 保存任意类型的值，并在 TTL 到期后自动在读取时失效。
//   - 适合极短生命周期、可容忍短暂过期窗口的热点数据，例如请求链路中的版本号、本地投影或只读配置片段。
//   - 缓存不启动后台 goroutine，也不做容量淘汰；高基数或长 TTL 场景应使用带容量控制的专用缓存实现。
//
// 使用示例：
//
//	cache := localcache.New[string, int64](time.Second)
//	cache.Set("user-1", 7)
//	version, ok := cache.Get("user-1")
//	if ok {
//		_ = version
//	}
//	cache.Delete("user-1")
type Cache[K comparable, V any] struct {
	ttl    time.Duration
	now    func() time.Time
	values sync.Map
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// New 创建一个进程内短 TTL 缓存。
//
// 用途：
//   - 创建可并发访问的本地缓存实例，用于在单个进程内复用短时间有效的数据。
//
// 参数说明：
//   - ttl：每个缓存项从写入开始的有效期；ttl 小于等于 0 时会回退为 1 秒，避免创建永久缓存项。
//
// 返回值说明：
//   - *Cache[K, V]：可直接调用 Get、Set、Delete 的缓存实例。
//
// 使用示例：
//
//	cache := localcache.New[string, int64](500 * time.Millisecond)
//	cache.Set("token-version:user-1", 3)
//	version, ok := cache.Get("token-version:user-1")
func New[K comparable, V any](ttl time.Duration) *Cache[K, V] {
	if ttl <= 0 {
		ttl = time.Second
	}
	return &Cache[K, V]{ttl: ttl, now: time.Now}
}

// Get 读取缓存项，并在缓存不存在或过期时返回未命中。
//
// 用途：
//   - 在访问远端缓存、数据库或其他昂贵依赖前，先尝试读取本进程内的短期值。
//   - 如果缓存项已过期，Get 会删除该项并返回未命中。
//
// 参数说明：
//   - key：缓存键，类型必须与创建 Cache 时的 K 一致。
//
// 返回值说明：
//   - V：缓存命中时返回对应值；未命中时返回 V 的零值。
//   - bool：true 表示命中且未过期；false 表示不存在、类型异常或已过期。
//
// 使用示例：
//
//	cache := localcache.New[string, int64](time.Second)
//	cache.Set("user-1", 7)
//	if version, ok := cache.Get("user-1"); ok {
//		_ = version
//	}
func (c *Cache[K, V]) Get(key K) (V, bool) {
	var zero V
	value, ok := c.values.Load(key)
	if !ok {
		return zero, false
	}
	cacheEntry, ok := value.(entry[V])
	if !ok {
		c.values.Delete(key)
		return zero, false
	}
	if !c.now().Before(cacheEntry.expiresAt) {
		c.values.Delete(key)
		return zero, false
	}
	return cacheEntry.value, true
}

// Set 写入或覆盖缓存项。
//
// 用途：
//   - 在远端缓存或数据库加载成功后，把结果写入本进程缓存，供后续短时间内的同 key 请求复用。
//
// 参数说明：
//   - key：缓存键，类型必须与创建 Cache 时的 K 一致。
//   - value：需要缓存的值，类型必须与创建 Cache 时的 V 一致。
//
// 返回值说明：
//   - Set 不返回值；写入后缓存项会在构造 Cache 时指定的 TTL 到期。
//
// 使用示例：
//
//	cache := localcache.New[string, int64](time.Second)
//	cache.Set("user-1", 7)
func (c *Cache[K, V]) Set(key K, value V) {
	c.values.Store(key, entry[V]{value: value, expiresAt: c.now().Add(c.ttl)})
}

// Delete 删除缓存项。
//
// 用途：
//   - 当调用方知道某个 key 对应的数据已变更或被撤销时，主动删除本进程缓存，避免等待 TTL 自然过期。
//
// 参数说明：
//   - key：需要删除的缓存键，类型必须与创建 Cache 时的 K 一致。
//
// 返回值说明：
//   - Delete 不返回值；key 不存在时也不会报错。
//
// 使用示例：
//
//	cache := localcache.New[string, int64](time.Second)
//	cache.Set("user-1", 7)
//	cache.Delete("user-1")
func (c *Cache[K, V]) Delete(key K) {
	c.values.Delete(key)
}

func (c *Cache[K, V]) setNowForTest(now func() time.Time) {
	c.now = now
}
