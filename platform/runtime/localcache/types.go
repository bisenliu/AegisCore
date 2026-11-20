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
