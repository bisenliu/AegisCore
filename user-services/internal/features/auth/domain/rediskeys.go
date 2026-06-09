package domain

import (
	"strings"

	"github.com/aegiscore/common/runtime/config"
)

// RedisKeyBuilder 构建认证会话 Redis key，并支持可选 app name 命名空间前缀。
type RedisKeyBuilder struct {
	appName string
}

// NewRedisKeyBuilder 根据配置的 app name 构造 key builder。
func NewRedisKeyBuilder(cfg *config.Config) RedisKeyBuilder {
	return RedisKeyBuilder{appName: strings.TrimSpace(cfg.App.Name)}
}

// AuthSession 返回一个 refresh token 会话载荷的 key。
func (b RedisKeyBuilder) AuthSession(userID string, sessionID string) string {
	return b.AuthSessionPrefix(userID) + sessionID
}

// AuthSessionPrefix 返回同一用户 refresh token 会话载荷 key 的前缀。
func (b RedisKeyBuilder) AuthSessionPrefix(userID string) string {
	return b.join("auth", "session", redisHashTag(userID)) + ":"
}

// AuthUserTokenVersion 返回一个用户 token version 缓存的 key。
func (b RedisKeyBuilder) AuthUserTokenVersion(userID string) string {
	return b.join("auth", "user", "token_version", redisHashTag(userID))
}

// AuthUserSessions 返回一个用户活跃会话 sorted-set 索引的 key。
func (b RedisKeyBuilder) AuthUserSessions(userID string) string {
	return b.join("auth", "user", "sessions", redisHashTag(userID))
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

func redisHashTag(userID string) string {
	return "{" + userID + "}"
}
