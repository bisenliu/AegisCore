package localcache_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/aegiscore/common/runtime/localcache"
)

// ExampleLoadingCache 展示 bounded fixed-TTL loading cache 的基本构造和读取。
// 第一次 Get 回源，后续同 key Get 在固定 TTL 内直接命中缓存。
func ExampleLoadingCache() {
	var loads atomic.Uint64
	cache, err := localcache.NewLoadingCache(localcache.Config{
		Name:        "user_profile",
		Capacity:    100,
		TTL:         time.Minute,
		LoadTimeout: time.Second,
	}, func(_ context.Context, key string) (string, error) {
		loads.Add(1)
		return "profile:" + key, nil
	})
	if err != nil {
		panic(err)
	}

	first, err := cache.Get(context.Background(), "alice")
	if err != nil {
		panic(err)
	}
	second, err := cache.Get(context.Background(), "alice")
	if err != nil {
		panic(err)
	}
	fmt.Println(first, second, loads.Load())

	// Output: profile:alice profile:alice 1
}

// ExampleLoadingCache_Get_concurrentMiss 展示同 key 并发 miss 的 singleflight 合并。
// 每个公开 Get 都计入一次 Miss，但共享 loader 只执行并记录一次加载结果。
func ExampleLoadingCache_Get_concurrentMiss() {
	var loads atomic.Uint64
	loaderStarted := make(chan struct{})
	releaseLoader := make(chan struct{})
	cache, err := localcache.NewLoadingCache(exampleCacheConfig("user_profile"), func(_ context.Context, key string) (string, error) {
		if loads.Add(1) == 1 {
			close(loaderStarted)
		}
		<-releaseLoader
		return "profile:" + key, nil
	})
	if err != nil {
		panic(err)
	}

	results := make(chan string, 2)
	for range 2 {
		go func() {
			value, loadErr := cache.Get(context.Background(), "alice")
			if loadErr != nil {
				panic(loadErr)
			}
			results <- value
		}()
	}
	<-loaderStarted

	// 等到两个公开 Get 都登记 miss，证明第二个调用已经参与同一轮共享加载。
	waitForCacheExampleCondition(func() bool { return cache.Stats().Miss == 2 })
	close(releaseLoader)
	first, second := <-results, <-results
	fmt.Println(first == second, loads.Load(), cache.Stats().Miss)

	// Output: true 1 2
}

// ExampleLoadingCache_Invalidate 展示权威数据写入成功后的精确 key 失效。
// 必须先完成权威写入再调用 Invalidate，避免操作间隙重新缓存旧值。
func ExampleLoadingCache_Invalidate() {
	var source atomic.Int64
	source.Store(1)
	cache, err := localcache.NewLoadingCache(exampleCacheConfig("user_profile"), func(context.Context, string) (int64, error) {
		return source.Load(), nil
	})
	if err != nil {
		panic(err)
	}

	before, err := cache.Get(context.Background(), "alice")
	if err != nil {
		panic(err)
	}
	// 模拟 repository.Update 已成功提交新值，然后同步失效本地旧值。
	source.Store(2)
	cache.Invalidate("alice")
	after, err := cache.Get(context.Background(), "alice")
	if err != nil {
		panic(err)
	}
	fmt.Println(before, after)

	// Output: 1 2
}

// ExampleLoadingCache_Get_invalidationConflict 展示连续两次加载都遇到并发失效时的 fail-closed 行为。
// 第一次 revision 冲突会透明重试；第二次仍冲突时 Get 返回 ErrInvalidated，旧值不会发布或返回。
func ExampleLoadingCache_Get_invalidationConflict() {
	var calls atomic.Uint64
	started := make(chan uint64, 2)
	releases := []chan struct{}{make(chan struct{}), make(chan struct{})}
	cache, err := localcache.NewLoadingCache(exampleCacheConfig("user_profile"), func(context.Context, string) (int, error) {
		call := calls.Add(1)
		started <- call
		<-releases[call-1]
		return int(call), nil
	})
	if err != nil {
		panic(err)
	}

	result := make(chan error, 1)
	go func() {
		_, loadErr := cache.Get(context.Background(), "alice")
		result <- loadErr
	}()

	firstCall := <-started
	cache.Invalidate("alice")
	close(releases[firstCall-1])
	secondCall := <-started
	cache.Invalidate("alice")
	close(releases[secondCall-1])

	fmt.Println(errors.Is(<-result, localcache.ErrInvalidated), calls.Load())

	// Output: true 2
}

// ExampleLoadingCache_Stats 展示请求、加载和容量驱逐的累计统计。
// 容量驱逐在导致驱逐的 Get 返回前同步可见；Invalidate 和 TTL 到期不计入该字段。
func ExampleLoadingCache_Stats() {
	cache, err := localcache.NewLoadingCache(localcache.Config{
		Name:        "user_profile",
		Capacity:    1,
		TTL:         time.Minute,
		LoadTimeout: time.Second,
	}, func(_ context.Context, key string) (string, error) {
		return "profile:" + key, nil
	})
	if err != nil {
		panic(err)
	}

	_, _ = cache.Get(context.Background(), "alice") // miss + load success
	_, _ = cache.Get(context.Background(), "alice") // hit
	_, _ = cache.Get(context.Background(), "bob")   // miss + load success + capacity eviction
	stats := cache.Stats()
	fmt.Println(stats.Hit, stats.Miss, stats.LoadSuccess)
	fmt.Println(stats.CapacityEvictions, stats.Capacity)

	// Output:
	// 1 2 2
	// 1 1
}

// ExampleLoadingCache_mutableValue 展示缓存 slice、map 或 pointer 时的调用方复制责任。
// LoadingCache 不执行 deep clone；修改前必须复制，避免污染缓存内容或引入 data race。
func ExampleLoadingCache_mutableValue() {
	cache, err := localcache.NewLoadingCache(exampleCacheConfig("rbac_user_roles"), func(context.Context, string) ([]string, error) {
		return []string{"admin"}, nil
	})
	if err != nil {
		panic(err)
	}

	cachedRoles, err := cache.Get(context.Background(), "alice")
	if err != nil {
		panic(err)
	}
	roles := append([]string(nil), cachedRoles...)
	roles[0] = "editor"

	cachedAgain, err := cache.Get(context.Background(), "alice")
	if err != nil {
		panic(err)
	}
	fmt.Println(roles[0], cachedAgain[0])

	// Output: editor admin
}

func exampleCacheConfig(name string) localcache.Config {
	return localcache.Config{
		Name:        name,
		Capacity:    10,
		TTL:         time.Minute,
		LoadTimeout: time.Second,
	}
}

func waitForCacheExampleCondition(condition func() bool) {
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			panic("timed out waiting for cache example condition")
		}
		runtime.Gosched()
	}
}
