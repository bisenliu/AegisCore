package localcache

import (
	"context"
	"time"
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
