package auth

import (
	"errors"
	"time"

	"github.com/aegiscore/user-services/internal/user"
	"github.com/google/uuid"
)

// ErrAuthSessionNotFound 表示 refresh token 会话不存在或已过期。
var ErrAuthSessionNotFound = errors.New("auth session not found")

// ErrAuthSessionMismatch 表示 refresh token 会话与预期用户或版本不一致。
var ErrAuthSessionMismatch = errors.New("auth session mismatch")

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

// UserCredential 是认证能力需要的最小用户凭据模型。
type UserCredential struct {
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       user.UserStatus
	TokenVersion int64
}

// CanLogin 返回当前状态是否允许普通认证。
func (u UserCredential) CanLogin() bool {
	return u.Status.CanLogin()
}

// RequiresPasswordChange 返回用户是否必须完成强制改密流程。
func (u UserCredential) RequiresPasswordChange() bool {
	return u.Status == user.UserStatusMustChangePassword
}

// CanChangePassword 返回用户是否可通过受限 token 流程修改密码。
func (u UserCredential) CanChangePassword() bool {
	return u.RequiresPasswordChange()
}

// CredentialUpdateResult 返回凭证替换后的用户和 token version。
type CredentialUpdateResult struct {
	UserID       uuid.UUID
	TokenVersion int64
}

// UpdateCredentialsInput 包含改密时使用的新凭证和目标状态。
type UpdateCredentialsInput struct {
	UserID       uuid.UUID
	PasswordHash string
	Status       user.UserStatus
}

// SessionRevocationResult 返回撤销用户全部会话后的新 token version。
type SessionRevocationResult struct {
	UserID       uuid.UUID
	TokenVersion int64
}
