package service

import (
	"strings"

	"github.com/aegiscore/common/runtime/config"
)

type RedisKeyBuilder struct {
	appName string
}

func NewRedisKeyBuilder(cfg *config.Config) RedisKeyBuilder {
	return RedisKeyBuilder{appName: strings.TrimSpace(cfg.App.Name)}
}

func (b RedisKeyBuilder) AuthSession(sessionID string) string {
	return b.join("auth", "session", sessionID)
}

func (b RedisKeyBuilder) AuthUserTokenVersion(userID string) string {
	return b.join("auth", "user", userID, "token_version")
}

func (b RedisKeyBuilder) AuthUserSessions(userID string) string {
	return b.join("auth", "user", userID, "sessions")
}

func (b RedisKeyBuilder) join(parts ...string) string {
	if b.appName == "" {
		return strings.Join(parts, ":")
	}

	all := make([]string, 0, len(parts)+1)
	all = append(all, b.appName)
	all = append(all, parts...)
	return strings.Join(all, ":")
}
