package app

import (
	"context"
	"time"

	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	"github.com/google/uuid"
)

// AuthService 定义认证、刷新、改密和登出用例。
type AuthService interface {
	Login(ctx context.Context, cmd LoginCommand) (*TokenResult, error)
	ChangePassword(ctx context.Context, cmd ChangePasswordCommand) (*ChangePasswordResult, error)
	Refresh(ctx context.Context, cmd RefreshTokenCommand) (*TokenResult, error)
	Logout(ctx context.Context) (*LogoutResult, error)
	LogoutAll(ctx context.Context) (*LogoutResult, error)
}

// UserCredentialStore 定义认证流程对用户凭据的最小依赖面。
type UserCredentialStore interface {
	GetByUsername(ctx context.Context, username string) (*authdomain.UserCredential, error)
	GetCredentialByUserID(ctx context.Context, userID uuid.UUID) (*authdomain.UserCredential, error)
	UpdateCredentials(ctx context.Context, input authdomain.UpdateCredentialsInput) (int64, error)
}

// UserTokenVersionStore 定义认证失效控制对用户 token version 的最小依赖面。
type UserTokenVersionStore interface {
	GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
}

// AuthSessionStore 管理 token version 缓存和 refresh token 会话撤销状态。
type AuthSessionStore interface {
	GetCachedTokenVersion(ctx context.Context, userID string) (int64, error)
	CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
	DeleteCachedTokenVersion(ctx context.Context, userID string) error
	CreateSession(ctx context.Context, session authdomain.AuthSession, ttl time.Duration) error
	RotateSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, ttl time.Duration) error
	GetSession(ctx context.Context, userID string, sessionID string) (authdomain.AuthSession, error)
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	DeleteAllUserSessions(ctx context.Context, userID string) error
}

// AuthSessionLifecycle 为服务用例创建、校验和撤销认证会话。
type AuthSessionLifecycle interface {
	CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error
	ValidatePasswordChangeClaims(ctx context.Context, claims *commonauth.Claims) error
	ValidateRefreshSession(ctx context.Context, claims *commonauth.Claims) (authdomain.AuthSession, int64, error)
	RotateTokenSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, refreshTTL time.Duration) error
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	RevokeUserSessionsAtVersion(ctx context.Context, userID uuid.UUID, tokenVersion int64) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*authdomain.SessionRevocationResult, error)
}

// TokenVersionValidator 是面向中间件的契约，用于拒绝过期 access token。
type TokenVersionValidator interface {
	ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
}

// CredentialVerifier 校验登录凭证并完成强制改密。
type CredentialVerifier interface {
	VerifyPassword(ctx context.Context, username string, plainPassword string) (*authdomain.UserCredential, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, newPassword string) (*authdomain.CredentialUpdateResult, error)
}
