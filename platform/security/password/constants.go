package password

import stderrors "errors"

const (
	// defaultBcryptCost 定义密码加密计算的强度系数（默认为 12），在保障密码防破解安全性的同时避免占用过多服务器 CPU 资源。
	defaultBcryptCost = 12
	// maxPasswordLength 规定明文密码的最大长度为 72 个字节（受底层加密算法限制，超过 72 字节的部分会被算法自动忽略，因此直接拒绝以避免安全隐患）。
	maxPasswordLength = 72
	// maxEncodedHashLength 限制密文哈希字符串的最大长度，防止恶意传入超长字符串浪费内存与计算资源。
	maxEncodedHashLength = 128
)

var (
	// ErrEmptyPassword 表示哈希或校验收到空明文密码。
	ErrEmptyPassword = stderrors.New("password is empty")
	// ErrPasswordTooLong 表示明文密码超过包允许的最大长度。
	ErrPasswordTooLong = stderrors.New("password is too long")
	// ErrInvalidHash 表示编码后的密码哈希格式错误或不受支持。
	ErrInvalidHash = stderrors.New("password hash is invalid")
)
