package password

import (
	"errors"

	"golang.org/x/crypto/argon2"
)

const (
	// passwordAlgorithm 和 passwordVersion 会写入编码后的哈希，用于兼容性校验。
	passwordAlgorithm = "argon2id"
	passwordVersion   = argon2.Version

	// passwordMemory 是 Argon2id 内存成本，单位为 KiB。
	passwordMemory uint32 = 64 * 1024
	// passwordIterations 是 Argon2id 时间成本。
	passwordIterations uint32 = 3
	// passwordParallelism 是 Argon2id 并行度成本。
	passwordParallelism uint8 = 4
	// passwordSaltLength 是随机盐长度，单位为字节。
	passwordSaltLength = 16
	// passwordKeyLength 是派生密钥长度，单位为字节。
	passwordKeyLength uint32 = 32
	// maxEncodedHashLength 限制不可信编码哈希输入的解析成本。
	maxEncodedHashLength = 512
	// maxPasswordLength 限制明文密码长度，避免超大输入放大请求内存和解析成本。
	maxPasswordLength = 256

	// defaultArgon2Concurrency 是单进程默认 Argon2id 执行并发上限。
	defaultArgon2Concurrency = 2
	// defaultArgon2QueueSize 是单进程默认 Argon2id KDF 区域请求总数上限，包含执行中和等待中的请求。
	defaultArgon2QueueSize = 16
)

var (
	// ErrEmptyPassword 表示哈希或校验收到空明文密码。
	ErrEmptyPassword = errors.New("password is empty")
	// ErrPasswordTooLong 表示明文密码超过包允许的最大长度。
	ErrPasswordTooLong = errors.New("password is too long")
	// ErrInvalidHash 表示编码后的密码哈希格式错误或不受支持。
	ErrInvalidHash = errors.New("password hash is invalid")
	// ErrPasswordKDFBusy 表示密码 KDF 执行中和等待执行的请求总数已达上限。
	ErrPasswordKDFBusy = errors.New("password kdf is busy")
)
