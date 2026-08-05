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

// NewLoadingCache 创建一个有容量上限、固定 TTL、支持并发回源合并的进程内缓存。
//
// 配置项含义如下：
//   - Name 是缓存实例的稳定名称，会裁剪首尾空白，不能为空。它适合用作 metrics 中的低基数
//     cache label，不应包含 user ID、请求 ID 等动态内容。
//   - Capacity 是最多保留的 item 数量，必须大于零。达到上限后缓存会驱逐 item；它限制的是
//     item 数量，不是 value 的内存字节数。
//   - TTL 是 value 从成功写入缓存开始计算的固定有效期，必须大于零。命中 Get 不会延长 TTL，
//     因此它不是 sliding expiration。
//   - LoadTimeout 是一次共享 loader 调用的最长时间，必须大于零。超时通过 context 通知 loader，
//     loader 必须主动检查 ctx.Done() 或把 ctx 传给下游调用；缓存不会强制终止 goroutine。
//   - loader 在缓存 miss 时按 key 回源，不能为空。成功结果会写入缓存，错误会原样返回且不会缓存。
//
// 用法一：创建缓存并读取数据。
//
// 下例把用户资料保留 5 分钟，最多缓存 10,000 个用户。第一次读取某个 userID 时调用
// repository，后续命中直接返回缓存值；写入 5 分钟后即使期间持续命中，该值仍会过期。
//
//	type UserProfile struct {
//		ID   string
//		Name string
//	}
//
//	profileCache, err := localcache.NewLoadingCache[UserProfile](localcache.Config{
//		Name:        "user_profile",
//		Capacity:    10_000,
//		TTL:         5 * time.Minute,
//		LoadTimeout: 2 * time.Second,
//	}, func(ctx context.Context, userID string) (UserProfile, error) {
//		return profileRepository.FindByID(ctx, userID)
//	})
//	if err != nil {
//		return err
//	}
//
//	profile, err := profileCache.Get(ctx, userID)
//	if err != nil {
//		return err
//	}
//	useProfile(profile)
//
// key 不会被裁剪、解析或规范化，调用方应在 Get 和 Invalidate 前生成同一种稳定字符串；例如同一
// UUID 不要混用大小写或带空白的形式。Get 接受 nil context，并把它视为 context.Background，
// 但请求路径通常应传入真实 context，以便调用方停止等待。
//
// 用法二：理解并发 miss 和调用方取消。
//
// 同一时刻有多个 goroutine Get 同一个 miss key 时，只执行一次 loader，所有仍在等待的调用方共享
// 该结果；不同 key 的 loader 可以并发执行。每个 Get 都会分别记录一次 miss，但共享回源成功或失败
// 只记录一次 load result。
//
// loader 使用最先启动本轮回源的调用方 context 作为 value 来源，但会移除该 context 原有的取消和
// deadline，再施加 Config.LoadTimeout。这样某个调用方超时或取消时，它自己的 Get 会立即返回
// ctx.Err()，却不会取消其他调用方正在共享的回源。即使所有调用方都已取消，已经开始的 loader 仍可
// 运行到完成或 LoadTimeout，并在成功时填充缓存。
//
// 因此 loader 不应把 context 之外的请求局部对象带到异步生命周期中，也不应依赖某个调用方的
// deadline；需要限制数据库、Redis 或 HTTP 请求时，应把收到的 loader context 继续传给下游。
//
// 用法三：在业务写入成功后主动失效。
//
//	if err := profileRepository.Update(ctx, command); err != nil {
//		return err
//	}
//	profileCache.Invalidate(command.UserID)
//
// Invalidate 同步删除指定 key；InvalidateAll 同步删除当前全部 item，适合配置整体切换等少数场景。
// 二者都不强制停止已经运行的 loader，而是通过实例级 revision 禁止失效前启动的回源结果重新发布
// 旧值。由于 revision 属于整个 cache，一次 Invalidate(key) 只删除该 key 的已缓存 value，但也会
// 抑制同一实例中更早开始、尚未发布的其他 key 回源结果。如果第一次回源与失效发生冲突，Get 会自动
// 再回源一次；若第二次回源也再次遭遇失效，则返回 ErrInvalidated，由上层选择失败、稍后重试或重新读取。
//
//	profileCache.Invalidate(userID)
//	profile, err := profileCache.Get(ctx, userID)
//	if errors.Is(err, localcache.ErrInvalidated) {
//		return retryLater(err)
//	}
//	if err != nil {
//		return err
//	}
//
// Invalidate 应在权威数据写入成功后调用；若先失效后写入，两个操作之间的读取仍可能再次缓存旧值。
// InvalidateAll 的影响范围更大，频繁调用会降低命中率并抑制同时进行的回源，不能替代精确 key 失效。
//
// 用法四：区分 loader 错误、回源超时和调用方取消。
//
//   - loader 返回的数据库、Redis 或业务错误会直接返回给当前共享调用方，LoadError 加一；该错误不会
//     进入缓存，下一次 Get 会重新回源。
//   - loader 超过 LoadTimeout 且正确响应 context 时，通常返回 context.DeadlineExceeded，并计入
//     LoadError。
//   - 调用方 context 先取消时，该 Get 返回调用方的 context.Canceled 或
//     context.DeadlineExceeded；共享 loader 不因此取消，其最终结果仍决定 LoadSuccess/LoadError。
//   - ErrInvalidated 表示连续两次成功回源都因并发失效而没有发布，不是 loader 自身错误。
//
// 用法五：读取累计统计。
//
//	stats := profileCache.Stats()
//	log.Info("local cache snapshot",
//		zap.String("cache", profileCache.Name()),
//		zap.Uint64("hit", stats.Hit),
//		zap.Uint64("miss", stats.Miss),
//		zap.Uint64("load_success", stats.LoadSuccess),
//		zap.Uint64("load_error", stats.LoadError),
//		zap.Uint64("capacity_evictions", stats.CapacityEvictions),
//		zap.Uint64("capacity", stats.Capacity),
//	)
//
// Stats 返回从实例创建至今的累计快照，不会重置计数。各字段分别使用原子计数，并发读写安全，但在
// 高并发更新期间不保证所有字段对应完全相同的时间点。LoadSuccess 表示 loader 成功返回的次数，也
// 包含后来因 revision 冲突而未发布的结果。CapacityEvictions 只统计容量达到上限造成的驱逐，不包含
// TTL 到期、Invalidate 或 InvalidateAll。LoadingCache 同时实现 StatsSource，可以直接交给
// metrics collector；指标 label 应使用 Name，不要使用原始 cache key。
//
// 用法六：缓存可变 value 时由调用方负责复制。
//
// LoadingCache 存储和返回 V 本身。若 V 是 pointer、map、slice 或内部包含可变引用，Get 不会深拷贝；
// 任意调用方原地修改 value 都可能改变其他调用方看到的缓存内容，并产生 data race。优先缓存不可变
// value；确需缓存可变结构时，在对外返回前复制，并且不要把缓存内对象直接交给会修改它的代码。
//
//	cachedRoles, err := roleCache.Get(ctx, userID)
//	if err != nil {
//		return nil, err
//	}
//	roles := append([]Role(nil), cachedRoles...)
//	return roles, nil
//
// LoadingCache 的公开方法可供多个 goroutine 并发调用，不需要 Start、Close 或后台清理生命周期。
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
