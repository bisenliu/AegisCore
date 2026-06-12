package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
)

// AuthSessionLifecycle 为 auth command use case 创建、校验和撤销认证会话。
type AuthSessionLifecycle interface {
	CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error
	ValidatePasswordChangeClaims(ctx context.Context, claims *commonauth.Claims) error
	ValidateRefreshSession(ctx context.Context, claims *commonauth.Claims) (authdomain.AuthSession, int64, error)
	RotateTokenSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, refreshTTL time.Duration) error
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	RevokeUserSessionsAtVersion(ctx context.Context, userID uuid.UUID, tokenVersion int64) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*authdomain.SessionRevocationResult, error)
}

type authSessionLifecycle struct {
	users    authapplication.UserTokenVersionStore
	sessions authapplication.AuthSessionStore
}

func newAuthSessionLifecycle(users authapplication.UserTokenVersionStore, sessions authapplication.AuthSessionStore) AuthSessionLifecycle {
	return &authSessionLifecycle{users: users, sessions: sessions}
}

// CreateTokenSession 为新签发的 token pair 持久化 refresh 会话元数据。
func (m *authSessionLifecycle) CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error {
	if err := m.sessions.CreateSession(ctx, authdomain.AuthSession{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		logger.Error(ctx, "create auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return err
	}
	logger.Info(ctx, "auth session created", zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion))
	return nil
}

// ValidatePasswordChangeClaims 校验改密 token version 是否仍为当前版本。
func (m *authSessionLifecycle) ValidatePasswordChangeClaims(ctx context.Context, claims *commonauth.Claims) error {
	currentVersion, err := m.currentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "password change user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return userdomain.ErrUserNotFound
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
func (m *authSessionLifecycle) ValidateRefreshSession(ctx context.Context, claims *commonauth.Claims) (authdomain.AuthSession, int64, error) {
	session, err := m.sessions.GetSession(ctx, claims.UserID, claims.SessionID)
	if err != nil {
		if errors.Is(err, authdomain.ErrAuthSessionNotFound) {
			logger.Warn(ctx, "refresh session not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return authdomain.AuthSession{}, 0, authdomain.ErrTokenInvalid
		}
		logger.Error(ctx, "get refresh session failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return authdomain.AuthSession{}, 0, err
	}
	if err := authvalidators.ValidateRefreshSessionClaims(session, claims.UserID, claims.TokenVersion); err != nil {
		logger.Warn(ctx, "refresh session mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("session_token_version", session.TokenVersion), zap.Int64("token_version", claims.TokenVersion))
		return authdomain.AuthSession{}, 0, err
	}
	currentVersion, err := m.currentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "refresh user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return authdomain.AuthSession{}, 0, userdomain.ErrUserNotFound
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
func (m *authSessionLifecycle) RotateTokenSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, refreshTTL time.Duration) error {
	if err := m.sessions.RotateSession(ctx, oldSession, newSession, refreshTTL); err != nil {
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
func (m *authSessionLifecycle) DeleteSession(ctx context.Context, userID string, sessionID string) error {
	if err := m.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		logger.Error(ctx, "delete auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Error(err))...)
		return err
	}
	logger.Info(ctx, "auth session deleted", zap.String("user_id", userID), zap.String("session_id", sessionID))
	return nil
}

func (m *authSessionLifecycle) currentTokenVersion(ctx context.Context, userID string) (int64, error) {
	return authvalidators.Current(ctx, m.users, m.sessions, userID)
}

// RevokeAllUserSessions 递增 token version、刷新缓存并删除全部 refresh 会话。
func (m *authSessionLifecycle) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*authdomain.SessionRevocationResult, error) {
	tokenVersion, err := m.users.IncrementTokenVersion(ctx, userID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "revoke all user sessions user not found", zap.String("user_id", userID.String()))
			return nil, userdomain.ErrUserNotFound
		}
		logger.Error(ctx, "increment token version failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, err
	}
	if err := m.RevokeUserSessionsAtVersion(ctx, userID, tokenVersion); err != nil {
		logger.Error(ctx, "revoke all user sessions projection failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
	}
	logger.Info(ctx, "all user sessions revoked", zap.String("user_id", userID.String()), zap.Int64("token_version", tokenVersion))
	return &authdomain.SessionRevocationResult{UserID: userID, TokenVersion: tokenVersion}, nil
}

// RevokeUserSessionsAtVersion 刷新 token version 投影并删除全部 refresh 会话，不修改 PostgreSQL 版本。
func (m *authSessionLifecycle) RevokeUserSessionsAtVersion(ctx context.Context, userID uuid.UUID, tokenVersion int64) error {
	userIDString := userID.String()
	var projectionErr error
	if err := m.sessions.CacheTokenVersion(ctx, userIDString, tokenVersion); err != nil {
		logger.Error(ctx, "refresh token version cache failed", logger.StackTrace(zap.String("user_id", userIDString), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		projectionErr = errors.Join(projectionErr, fmt.Errorf("refresh token version cache: %w", err))
		if evictErr := m.sessions.DeleteCachedTokenVersion(ctx, userIDString); evictErr != nil {
			logger.Error(ctx, "delete token version cache failed", logger.StackTrace(zap.String("user_id", userIDString), zap.Int64("token_version", tokenVersion), zap.Error(evictErr))...)
			projectionErr = errors.Join(projectionErr, fmt.Errorf("delete token version cache: %w", evictErr))
		}
	}
	if err := m.sessions.DeleteAllUserSessions(ctx, userIDString); err != nil {
		logger.Error(ctx, "delete all user sessions failed", logger.StackTrace(zap.String("user_id", userIDString), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		projectionErr = errors.Join(projectionErr, fmt.Errorf("delete all user sessions: %w", err))
	}
	return projectionErr
}
