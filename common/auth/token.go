package auth

import "strings"

func StripBearerPrefix(token string) string {
	token = strings.TrimSpace(token)
	if len(token) >= len(TokenPrefix) && strings.EqualFold(token[:len(TokenPrefix)], TokenPrefix) {
		return strings.TrimSpace(token[len(TokenPrefix):])
	}
	return token
}
