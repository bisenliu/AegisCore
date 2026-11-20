package auth

// AccessToken 包含共享 HTTP middleware 执行上下文注入和 token version 校验所需的最小认证结果。
type AccessToken struct {
	UserID       string
	SessionID    string
	TokenVersion int64
}

// AccessTokenVerifier 定义共享 HTTP middleware 对访问令牌校验器的最小依赖。
type AccessTokenVerifier interface {
	VerifyAccessToken(tokenString string) (AccessToken, error)
}
