package password

type passwordParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// Options 描述单个密码 KDF 服务实例的资源预算。
type Options struct {
	Concurrency int
	QueueSize   int
}

// Service 提供 Argon2id 密码哈希和校验能力，并用实例级门控限制 KDF 资源占用。
type Service struct {
	params passwordParams
	gate   chan struct{}
	queue  chan struct{}
}

var defaultPasswordParams = passwordParams{
	memory:      passwordMemory,
	iterations:  passwordIterations,
	parallelism: passwordParallelism,
	saltLength:  passwordSaltLength,
	keyLength:   passwordKeyLength,
}
