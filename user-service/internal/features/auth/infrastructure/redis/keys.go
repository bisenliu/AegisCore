package redis

import (
	"github.com/aegiscore/common/runtime/rediskey"
)

// KeyCatalog 构造认证 Redis adapter 私有 key。
type KeyCatalog struct {
	builder rediskey.Builder
}

// NewKeyCatalog 根据 app name 构造认证 Redis key catalog。
func NewKeyCatalog(appName string) (KeyCatalog, error) {
	builder, err := rediskey.NewBuilder(rediskey.Options{Namespace: appName})
	if err != nil {
		return KeyCatalog{}, err
	}
	scoped, err := builder.Scoped("auth")
	if err != nil {
		return KeyCatalog{}, err
	}
	return KeyCatalog{builder: scoped}, nil
}

// AuthSession 返回一个 refresh token 会话载荷的 key。
func (c KeyCatalog) AuthSession(userID string, sessionID string) string {
	return c.builder.MustKey("session", rediskey.HashTag(userID), sessionID)
}

// PasswordChangeSession 返回一个强制改密一次性会话载荷的 key。
func (c KeyCatalog) PasswordChangeSession(userID string, sessionID string) string {
	return c.builder.MustKey("password_change_session", rediskey.HashTag(userID), sessionID)
}

// AuthSessionPrefix 返回同一用户 refresh token 会话载荷 key 的前缀。
func (c KeyCatalog) AuthSessionPrefix(userID string) string {
	return c.builder.MustPrefix("session", rediskey.HashTag(userID))
}

// AuthUserTokenVersion 返回一个用户 token version 缓存的 key。
func (c KeyCatalog) AuthUserTokenVersion(userID string) string {
	return c.builder.MustKey("user", "token_version", rediskey.HashTag(userID))
}

// AuthUserSessions 返回一个用户活跃会话 sorted-set 索引的 key。
func (c KeyCatalog) AuthUserSessions(userID string) string {
	return c.builder.MustKey("user", "sessions", rediskey.HashTag(userID))
}

// AuthUserSessionsPurge 返回用户活跃会话索引后台清理用的临时 key。
func (c KeyCatalog) AuthUserSessionsPurge(userID string, purgeID string) string {
	return c.builder.MustKey("user", "sessions", rediskey.HashTag(userID), "purge", purgeID)
}
