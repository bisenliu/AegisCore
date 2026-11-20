package auth

import (
	"context"
	"errors"
)

// TokenVersionValidator 校验 token version，使认证中间件可以拒绝已撤销或过期状态的 access token。
type TokenVersionValidator interface {
	ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
}

// ErrTokenVersionMismatch 表示 token 携带版本与服务端当前版本不一致。
var ErrTokenVersionMismatch = errors.New("token version mismatch")

// TokenVersionMismatchError 携带 token version mismatch 的结构化上下文。
type TokenVersionMismatchError struct {
	Current int64 // 服务端当前 token_version。
	Token   int64 // token claims 携带的 token_version。
}

// Error 返回稳定的 token version 不匹配错误消息。
func (e *TokenVersionMismatchError) Error() string {
	return ErrTokenVersionMismatch.Error()
}

// Unwrap 允许调用方通过 errors.Is 匹配 ErrTokenVersionMismatch。
func (e *TokenVersionMismatchError) Unwrap() error {
	return ErrTokenVersionMismatch
}

// ValidateTokenVersion 校验 token claims 版本是否等于服务端当前版本。
func ValidateTokenVersion(tokenVersion, currentVersion int64) error {
	if tokenVersion != currentVersion {
		return &TokenVersionMismatchError{Current: currentVersion, Token: tokenVersion}
	}
	return nil
}
