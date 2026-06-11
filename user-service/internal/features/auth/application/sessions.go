package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type authSessionLifecycle struct {
	users    UserTokenVersionStore
	sessions AuthSessionStore
}

type tokenVersionValidator struct {
	users    UserTokenVersionStore
	sessions AuthSessionStore
}

// NewTokenVersionValidator 构造由缓存和持久化存储支撑的 token version 校验器。
func NewTokenVersionValidator(users UserTokenVersionStore, sessions AuthSessionStore) commonauth.TokenVersionValidator {
	return &tokenVersionValidator{users: users, sessions: sessions}
}

func newAuthSessionLifecycle(users UserTokenVersionStore, sessions AuthSessionStore) AuthSessionLifecycle {
	return &authSessionLifecycle{users: users, sessions: sessions}
}

// CreateTokenSession 为新签发的 token pair 持久化 refresh 会话元数据。
func (m *authSessionLifecycle) CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error {
	if err := m.sessions.CreateSession(ctx, authdomain.AuthSession{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		logger.Error(ctx, "create auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return err
	}
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
	if currentVersion != claims.TokenVersion {
		logger.Warn(ctx, "password change token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("token_version", claims.TokenVersion))
		return authdomain.ErrTokenInvalid
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
	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		logger.Warn(ctx, "refresh session mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("session_token_version", session.TokenVersion), zap.Int64("token_version", claims.TokenVersion))
		return authdomain.AuthSession{}, 0, authdomain.ErrTokenInvalid
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
	if currentVersion != session.TokenVersion {
		logger.Warn(ctx, "refresh token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("session_token_version", session.TokenVersion))
		return authdomain.AuthSession{}, 0, authdomain.ErrTokenInvalid
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
	return nil
}

// DeleteSession 撤销一个 refresh token 会话。
func (m *authSessionLifecycle) DeleteSession(ctx context.Context, userID string, sessionID string) error {
	if err := m.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		logger.Error(ctx, "delete auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Error(err))...)
		return err
	}
	return nil
}

func (m *authSessionLifecycle) currentTokenVersion(ctx context.Context, userID string) (int64, error) {
	return currentTokenVersion(ctx, m.users, m.sessions, userID)
}

// ValidateTokenVersion 拒绝 version 不再匹配当前用户版本的 token。
func (v *tokenVersionValidator) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	currentVersion, err := currentTokenVersion(ctx, v.users, v.sessions, userID)
	if err != nil {
		return err
	}
	return commonauth.ValidateTokenVersion(tokenVersion, currentVersion)
}

func currentTokenVersion(ctx context.Context, users UserTokenVersionStore, sessions AuthSessionStore, userID string) (int64, error) {
	// access token 中间件对延迟敏感，因此先查 Redis，再回退到仓储。
	currentVersion, err := sessions.GetCachedTokenVersion(ctx, userID)
	if err == nil {
		return currentVersion, nil
	}
	if !errors.Is(err, authdomain.ErrTokenVersionCacheMiss) {
		logger.Error(ctx, "token version cache unavailable", logger.StackTrace(zap.String("user_id", userID), zap.Error(err))...)
		return 0, fmt.Errorf("get token version cache: %w", err)
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		// token 用户 ID 是外部 UUID，解析可保护仓储调用不接收畸形 claims。
		return 0, fmt.Errorf("parse user id: %w", err)
	}
	currentVersion, err = users.GetTokenVersion(ctx, parsedUserID)
	if err != nil {
		return 0, fmt.Errorf("get token version from database: %w", err)
	}
	if err := sessions.CacheTokenVersion(ctx, userID, currentVersion); err != nil {
		logger.Error(ctx, "backfill token version cache failed", logger.StackTrace(zap.String("user_id", userID), zap.Int64("token_version", currentVersion), zap.Error(err))...)
		return 0, fmt.Errorf("backfill token version cache: %w", err)
	}
	return currentVersion, nil
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
