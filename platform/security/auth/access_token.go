package auth

import "time"

// AccessToken 包含共享 HTTP middleware 执行上下文注入和 token version 校验所需的最小认证结果。
// ID 与 ExpiresAt 只描述 token 自身，不包含任何拒绝名单或业务吊销策略。
type AccessToken struct {
	ID           string
	UserID       string
	SessionID    string
	TokenVersion int64
	ExpiresAt    time.Time
}

// AccessTokenVerifier 定义共享 HTTP middleware 对访问令牌校验器的最小依赖。
type AccessTokenVerifier interface {
	VerifyAccessToken(tokenString string) (AccessToken, error)
}
