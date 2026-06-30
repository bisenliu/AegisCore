package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

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

// TokenVersionCache 管理 Redis token version 投影。
type TokenVersionCache interface {
	GetCachedTokenVersion(ctx context.Context, userID string) (int64, error)
	CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
	DeleteCachedTokenVersion(ctx context.Context, userID string) error
}

// RefreshSessionStore 管理 refresh token 会话生命周期。
type RefreshSessionStore interface {
	CreateSession(ctx context.Context, session authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error
	RotateSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error
	GetSession(ctx context.Context, userID string, sessionID string) (authdomain.AuthSession, error)
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	DeleteAllUserSessions(ctx context.Context, userID string) error
}
