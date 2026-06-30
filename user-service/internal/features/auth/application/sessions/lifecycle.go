package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

// Lifecycle 为 auth application use case 创建、校验和撤销认证会话。
type Lifecycle interface {
	CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error
	ValidatePasswordChangeClaims(ctx context.Context, claims *commonauth.Claims) error
	ValidateRefreshSession(ctx context.Context, claims *commonauth.Claims) (authdomain.AuthSession, int64, error)
	RotateTokenSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, refreshTTL time.Duration) error
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	CurrentTokenVersion(ctx context.Context, userID string) (int64, error)
	RevokeUserSessionsAtVersion(ctx context.Context, userID uuid.UUID, tokenVersion int64) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*authdomain.SessionRevocationResult, error)
}

type lifecycle struct {
	users                    authapplication.UserTokenVersionStore
	tokenVersions            authapplication.TokenVersionCache
	sessions                 authapplication.RefreshSessionStore
	maxActiveSessionsPerUser int
	localTokenVersions       authvalidators.TokenVersionLocalInvalidator
}

// NewLifecycle 构造认证会话生命周期组件。
func NewLifecycle(users authapplication.UserTokenVersionStore, tokenVersions authapplication.TokenVersionCache, sessions authapplication.RefreshSessionStore, maxActiveSessionsPerUser int, localTokenVersions ...authvalidators.TokenVersionLocalInvalidator) Lifecycle {
	lifecycle := &lifecycle{users: users, tokenVersions: tokenVersions, sessions: sessions, maxActiveSessionsPerUser: maxActiveSessionsPerUser}
	if len(localTokenVersions) > 0 {
		lifecycle.localTokenVersions = localTokenVersions[0]
	}
	return lifecycle
}

// CreateTokenSession 为新签发的 token pair 持久化 refresh 会话元数据。
func (m *lifecycle) CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error {
	if err := m.sessions.CreateSession(ctx, authdomain.AuthSession{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL, m.maxActiveSessionsPerUser); err != nil {
		logger.Error(ctx, "create auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return err
	}
	logger.Info(ctx, "auth session created", zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion))
	return nil
}

// ValidatePasswordChangeClaims 校验改密 token version 是否仍为当前版本。
func (m *lifecycle) ValidatePasswordChangeClaims(ctx context.Context, claims *commonauth.Claims) error {
	currentVersion, err := m.CurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			logger.Warn(ctx, "password change user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return identity.ErrUserNotFound
		}
		logger.Error(ctx, "get password change token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return err
	}
	if err := authvalidators.ValidateTokenVersionMatch(currentVersion, claims.TokenVersion); err != nil {
		logger.Warn(ctx, "password change token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("token_version", claims.TokenVersion))
		return err
	}
	return nil
}

// ValidateRefreshSession 校验会话存在性、claim 与会话一致性以及当前 token version。
func (m *lifecycle) ValidateRefreshSession(ctx context.Context, claims *commonauth.Claims) (authdomain.AuthSession, int64, error) {
	session, err := m.sessions.GetSession(ctx, claims.UserID, claims.SessionID)
	if err != nil {
		if errors.Is(err, authdomain.ErrAuthSessionNotFound) {
			logger.Warn(ctx, "refresh session not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return authdomain.AuthSession{}, 0, errors.Join(authdomain.ErrTokenInvalid, authdomain.ErrAuthSessionNotFound)
		}
		logger.Error(ctx, "get refresh session failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return authdomain.AuthSession{}, 0, err
	}
	if err := authvalidators.ValidateRefreshSessionClaims(session, claims.UserID, claims.TokenVersion); err != nil {
		logger.Warn(ctx, "refresh session mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("session_token_version", session.TokenVersion), zap.Int64("token_version", claims.TokenVersion))
		return authdomain.AuthSession{}, 0, err
	}
	currentVersion, err := m.CurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			logger.Warn(ctx, "refresh user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return authdomain.AuthSession{}, 0, identity.ErrUserNotFound
		}
		logger.Error(ctx, "get refresh token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return authdomain.AuthSession{}, 0, err
	}
	if err := authvalidators.ValidateTokenVersionMatch(currentVersion, session.TokenVersion); err != nil {
		logger.Warn(ctx, "refresh token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("session_token_version", session.TokenVersion))
		return authdomain.AuthSession{}, 0, err
	}
	return session, currentVersion, nil
}

// RotateTokenSession 原子消费旧 refresh 会话，并创建新 refresh 会话。
func (m *lifecycle) RotateTokenSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, refreshTTL time.Duration) error {
	if err := m.sessions.RotateSession(ctx, oldSession, newSession, refreshTTL, m.maxActiveSessionsPerUser); err != nil {
		if errors.Is(err, authdomain.ErrAuthSessionNotFound) || errors.Is(err, authdomain.ErrAuthSessionMismatch) {
			logger.Warn(ctx, "rotate auth session rejected", zap.String("user_id", oldSession.UserID), zap.String("old_session_id", oldSession.SessionID), zap.String("new_session_id", newSession.SessionID), zap.Error(err))
			return authdomain.ErrTokenInvalid
		}
		logger.Error(ctx, "rotate auth session failed", logger.StackTrace(zap.String("user_id", oldSession.UserID), zap.String("old_session_id", oldSession.SessionID), zap.String("new_session_id", newSession.SessionID), zap.Error(err))...)
		return err
	}
	logger.Info(ctx, "auth session rotated", zap.String("user_id", oldSession.UserID), zap.String("old_session_id", oldSession.SessionID), zap.String("new_session_id", newSession.SessionID), zap.Int64("token_version", newSession.TokenVersion))
	return nil
}

// DeleteSession 撤销一个 refresh token 会话。
func (m *lifecycle) DeleteSession(ctx context.Context, userID string, sessionID string) error {
	if err := m.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		logger.Error(ctx, "delete auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Error(err))...)
		return err
	}
	logger.Info(ctx, "auth session deleted", zap.String("user_id", userID), zap.String("session_id", sessionID))
	return nil
}

// CurrentTokenVersion 返回用户当前 token version，优先使用缓存并在 miss 时回源。
func (m *lifecycle) CurrentTokenVersion(ctx context.Context, userID string) (int64, error) {
	return authvalidators.Current(ctx, m.users, m.tokenVersions, userID)
}

func (m *lifecycle) invalidateLocalTokenVersion(userID string) {
	if m.localTokenVersions != nil {
		m.localTokenVersions.InvalidateTokenVersion(userID)
	}
}
