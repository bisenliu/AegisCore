package application

// TokenResult 是登录、刷新和强制改密登录的 transport-neutral token 输出。
type TokenResult struct {
	AccessToken            string
	RefreshToken           string
	TokenType              string
	ExpiresIn              int64
	PasswordChangeRequired bool
}

// ChangePasswordResult 表示改密 use case 是否完成。
type ChangePasswordResult struct {
	Changed bool
}

// LogoutResult 表示登出 use case 是否完成。
type LogoutResult struct {
	LoggedOut bool
}
