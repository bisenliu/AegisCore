package password

import "fmt"

// DefaultOptions 返回生产可用的默认密码 KDF 实例资源预算。
func DefaultOptions() Options {
	return Options{Concurrency: defaultArgon2Concurrency, QueueSize: defaultArgon2QueueSize}
}

// NewService 构造带独立 Argon2id 资源门控的密码 KDF 服务。
func NewService(opts Options) (*Service, error) {
	if opts.Concurrency <= 0 {
		return nil, fmt.Errorf("password argon2 concurrency must be > 0")
	}
	if opts.QueueSize <= 0 {
		return nil, fmt.Errorf("password argon2 queue size must be > 0")
	}
	if opts.QueueSize < opts.Concurrency {
		return nil, fmt.Errorf("password argon2 queue size must be >= concurrency")
	}

	return &Service{
		params: defaultPasswordParams,
		gate:   make(chan struct{}, opts.Concurrency),
		queue:  make(chan struct{}, opts.QueueSize),
	}, nil
}
