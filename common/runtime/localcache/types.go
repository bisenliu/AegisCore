package localcache

import (
	"context"
	"time"
)

// Loader 定义 string key 缓存 miss 后的回源函数。
type Loader[V any] func(context.Context, string) (V, error)

// Config 描述一个本地缓存实例。
type Config struct {
	Name        string
	Capacity    uint64
	TTL         time.Duration
	LoadTimeout time.Duration
}

// Stats 是 localcache 暴露给 metrics collector 的稳定统计快照。
type Stats struct {
	Hit               uint64
	Miss              uint64
	LoadSuccess       uint64
	LoadError         uint64
	CapacityEvictions uint64
	Capacity          uint64
}

// StatsSource 定义可导出 localcache 统计快照的类型。
type StatsSource interface {
	Name() string
	Stats() Stats
}
