package password

import stderrors "errors"

const (
	// defaultBcryptCost 是当前唯一支持的密码哈希成本。
	defaultBcryptCost = 12
	// maxPasswordLength 遵循 bcrypt 安全输入上限，避免超长密码截断语义。
	maxPasswordLength = 72
	// maxEncodedHashLength 限制不可信编码哈希输入长度。
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
