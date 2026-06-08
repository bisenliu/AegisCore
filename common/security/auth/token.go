package auth

import (
	"errors"
	"strings"
)

var (
	ErrMissingBearerPrefix = errors.New("bearer prefix is required")
	ErrEmptyBearerToken    = errors.New("bearer token is empty")
)

func ParseBearerAuthorization(header string) (string, error) {
	header = strings.TrimSpace(header)
	if strings.EqualFold(header, TokenTypeBearer) {
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

func StripBearerPrefix(token string) string {
	token = strings.TrimSpace(token)
	if len(token) >= len(TokenPrefix) && strings.EqualFold(token[:len(TokenPrefix)], TokenPrefix) {
		return strings.TrimSpace(token[len(TokenPrefix):])
	}
	return token
}
