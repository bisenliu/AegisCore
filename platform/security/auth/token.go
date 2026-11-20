package auth

import (
	"errors"
	"strings"
)

var (
	// ErrMissingBearerPrefix 表示 Authorization 请求头未以 Bearer 开头。
	ErrMissingBearerPrefix = errors.New("bearer prefix is required")
	// ErrEmptyBearerToken 表示存在 Bearer 前缀但后续没有 token 值。
	ErrEmptyBearerToken = errors.New("bearer token is empty")
)

// ParseBearerAuthorization 严格解析包含 Bearer token 的 Authorization 请求头。
func ParseBearerAuthorization(header string) (string, error) {
	header = strings.TrimSpace(header)
	if strings.EqualFold(header, TokenTypeBearer) {
		// 单独的 "Bearer" 使用了正确 scheme，但仍未携带凭证。
		return "", ErrEmptyBearerToken
	}
	if len(header) < len(TokenPrefix) || !strings.EqualFold(header[:len(TokenPrefix)], TokenPrefix) {
		return "", ErrMissingBearerPrefix
	}

	token := strings.TrimSpace(header[len(TokenPrefix):])
	if token == "" {
		return "", ErrEmptyBearerToken
	}
	return token, nil
}

// StripBearerPrefix 从类似 token 的输入中移除可选 Bearer 前缀。
func StripBearerPrefix(token string) string {
	token = strings.TrimSpace(token)
	if len(token) >= len(TokenPrefix) && strings.EqualFold(token[:len(TokenPrefix)], TokenPrefix) {
		return strings.TrimSpace(token[len(TokenPrefix):])
	}
	return token
}
