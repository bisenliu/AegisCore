package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuthSession 包含 Redis 存储的 refresh token 会话元数据。
type AuthSession struct {
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// PasswordChangeSession 包含 Redis 存储的一次性强制改密会话元数据。
type PasswordChangeSession struct {
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	TokenID      string    `json:"token_id"`
	TokenVersion int64     `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SessionRevocationResult 返回撤销用户全部会话后的新 token version。
type SessionRevocationResult struct {
	UserID       uuid.UUID
	TokenVersion int64
}
