package repository

import (
	"context"
	"errors"
	"time"
)

// ErrAuthSessionNotFound 表示 refresh token 会话不存在或已过期。
var ErrAuthSessionNotFound = errors.New("auth session not found")

// ErrTokenVersionCacheMiss 表示 token version 缓存未命中，需要从持久化存储回填。
var ErrTokenVersionCacheMiss = errors.New("token version cache miss")

// ErrTokenVersionMismatch 表示 token 携带了过期的用户 token version。
var ErrTokenVersionMismatch = errors.New("token version mismatch")

// AuthSession 包含 Redis 存储的 refresh token 会话元数据。
type AuthSession struct {
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// AuthSessionRepository 管理 token version 缓存和 refresh token 会话撤销状态。
type AuthSessionRepository interface {
	GetCachedTokenVersion(ctx context.Context, userID string) (int64, error)
	CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
	CreateSession(ctx context.Context, session AuthSession, ttl time.Duration) error
	GetSession(ctx context.Context, sessionID string) (AuthSession, error)
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	DeleteAllUserSessions(ctx context.Context, userID string) error
}
