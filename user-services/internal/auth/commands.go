package auth

// LoginCommand 是用户名密码认证的应用层输入。
type LoginCommand struct {
	Username string
	Password string
}

// RefreshTokenCommand 是换取 refresh token 的应用层输入。
type RefreshTokenCommand struct {
	RefreshToken string
}

// ChangePasswordCommand 是使用受限 token 完成强制改密的应用层输入。
type ChangePasswordCommand struct {
	Token       string
	NewPassword string
}
