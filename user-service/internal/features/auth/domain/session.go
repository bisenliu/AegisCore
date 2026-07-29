package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuthSession 包含 refresh token 会话元数据。
type AuthSession struct {
	UserID       uuid.UUID
	SessionID    string
	TokenVersion int64
	ExpiresAt    time.Time
}

// PasswordChangeSession 包含一次性强制改密会话元数据。
type PasswordChangeSession struct {
	UserID       uuid.UUID
	SessionID    string
	TokenID      string
	TokenVersion int64
	ExpiresAt    time.Time
}

// SessionRevocationResult 返回撤销用户全部会话后的新 token version。
type SessionRevocationResult struct {
	UserID       uuid.UUID
	TokenVersion int64
}
