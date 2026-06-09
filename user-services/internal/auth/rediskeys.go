package auth

import (
	"strings"

	"github.com/aegiscore/common/runtime/config"
)

// RedisKeyBuilder 构建 Redis key，并支持可选 app name 命名空间前缀。
type RedisKeyBuilder struct {
	appName string
}

// NewRedisKeyBuilder 根据配置的 app name 构造 key builder。
func NewRedisKeyBuilder(cfg *config.Config) RedisKeyBuilder {
	return RedisKeyBuilder{appName: strings.TrimSpace(cfg.App.Name)}
}

// AuthSession 返回一个 refresh token 会话载荷的 key。
func (b RedisKeyBuilder) AuthSession(sessionID string) string {
	return b.join("auth", "session", sessionID)
}

// AuthUserTokenVersion 返回一个用户 token version 缓存的 key。
func (b RedisKeyBuilder) AuthUserTokenVersion(userID string) string {
	return b.join("auth", "user", userID, "token_version")
}

// AuthUserSessions 返回一个用户活跃会话 sorted-set 索引的 key。
func (b RedisKeyBuilder) AuthUserSessions(userID string) string {
	return b.join("auth", "user", userID, "sessions")
}

func (b RedisKeyBuilder) join(parts ...string) string {
	if b.appName == "" {
		// 空 app name 保留无前缀 key，用于本地测试和兼容已有部署。
		return strings.Join(parts, ":")
	}

	all := make([]string, 0, len(parts)+1)
	all = append(all, b.appName)
	all = append(all, parts...)
	return strings.Join(all, ":")
}
