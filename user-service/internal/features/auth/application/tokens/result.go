package tokens

// TokenResult 是登录、刷新和强制改密登录的 transport-neutral token 输出。
type TokenResult struct {
	AccessToken            string
	RefreshToken           string
	TokenType              string
	ExpiresIn              int64
	PasswordChangeRequired bool
}
