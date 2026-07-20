package localcache

import "errors"

var (
	// ErrNameRequired 表示本地缓存缺少稳定实例名。
	ErrNameRequired = errors.New("localcache name is required")
	// ErrCapacityRequired 表示本地缓存容量预算必须为正数。
	ErrCapacityRequired = errors.New("localcache capacity must be positive")
	// ErrTTLRequired 表示本地缓存 TTL 必须为正数。
	ErrTTLRequired = errors.New("localcache ttl must be positive")
	// ErrLoadTimeoutRequired 表示本地缓存回源超时必须为正数。
	ErrLoadTimeoutRequired = errors.New("localcache load timeout must be positive")
	// ErrLoaderRequired 表示 loading cache 缺少回源函数。
	ErrLoaderRequired = errors.New("localcache loader is required")
	// ErrClosed 表示 loading cache 已停止并拒绝新操作。
	ErrClosed = errors.New("localcache loading cache is stopped")
)
