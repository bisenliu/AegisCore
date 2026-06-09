package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	"github.com/aegiscore/user-services/internal/messages"
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
func NewTokenVersionValidator(users UserTokenVersionStore, sessions AuthSessionStore) TokenVersionValidator {
	return &tokenVersionValidator{users: users, sessions: sessions}
}

func newAuthSessionLifecycle(users UserTokenVersionStore, sessions AuthSessionStore) AuthSessionLifecycle {
	return &authSessionLifecycle{users: users, sessions: sessions}
}

// CreateTokenSession 为新签发的 token pair 持久化 refresh 会话元数据。
func (m *authSessionLifecycle) CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error {
	if err := m.sessions.CreateSession(ctx, authdomain.AuthSession{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		logger.Error(ctx, "create auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return response.FromError(err)
	}
	return nil
}

// ValidatePasswordChangeClaims 校验改密 token version 是否仍为当前版本。
func (m *authSessionLifecycle) ValidatePasswordChangeClaims(ctx context.Context, claims *commonauth.Claims) error {
	currentVersion, err := m.currentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "password change user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "get password change token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return response.FromError(err)
	}
	if currentVersion != claims.TokenVersion {
		logger.Warn(ctx, "password change token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("token_version", claims.TokenVersion))
		return response.TokenInvalidError(messages.MissingSession)
	}
	return nil
}

// ValidateRefreshSession 校验会话存在性、claim 与会话一致性以及当前 token version。
func (m *authSessionLifecycle) ValidateRefreshSession(ctx context.Context, claims *commonauth.Claims) (authdomain.AuthSession, int64, error) {
	session, err := m.sessions.GetSession(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, authdomain.ErrAuthSessionNotFound) {
			logger.Warn(ctx, "refresh session not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return authdomain.AuthSession{}, 0, response.TokenInvalidError(messages.MissingSession)
		}
		logger.Error(ctx, "get refresh session failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return authdomain.AuthSession{}, 0, response.FromError(err)
	}
	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		logger.Warn(ctx, "refresh session mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("session_token_version", session.TokenVersion), zap.Int64("token_version", claims.TokenVersion))
		return authdomain.AuthSession{}, 0, response.TokenInvalidError(messages.MissingSession)
	}
	currentVersion, err := m.currentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "refresh user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return authdomain.AuthSession{}, 0, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "get refresh token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return authdomain.AuthSession{}, 0, response.FromError(err)
	}
	if currentVersion != session.TokenVersion {
		logger.Warn(ctx, "refresh token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("session_token_version", session.TokenVersion))
		return authdomain.AuthSession{}, 0, response.TokenInvalidError(messages.MissingSession)
	}
	return session, currentVersion, nil
}

// RotateTokenSession 原子消费旧 refresh 会话，并创建新 refresh 会话。
func (m *authSessionLifecycle) RotateTokenSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, refreshTTL time.Duration) error {
	if err := m.sessions.RotateSession(ctx, oldSession, newSession, refreshTTL); err != nil {
		if errors.Is(err, authdomain.ErrAuthSessionNotFound) || errors.Is(err, authdomain.ErrAuthSessionMismatch) {
			logger.Warn(ctx, "rotate auth session rejected", zap.String("user_id", oldSession.UserID), zap.String("old_session_id", oldSession.SessionID), zap.String("new_session_id", newSession.SessionID), zap.Error(err))
			return response.TokenInvalidError(messages.MissingSession)
		}
		logger.Error(ctx, "rotate auth session failed", logger.StackTrace(zap.String("user_id", oldSession.UserID), zap.String("old_session_id", oldSession.SessionID), zap.String("new_session_id", newSession.SessionID), zap.Error(err))...)
		return response.FromError(err)
	}
	return nil
}

// DeleteSession 撤销一个 refresh token 会话。
func (m *authSessionLifecycle) DeleteSession(ctx context.Context, userID string, sessionID string) error {
	if err := m.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		logger.Error(ctx, "delete auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Error(err))...)
		return response.FromError(err)
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
	if currentVersion != tokenVersion {
		return authdomain.ErrTokenVersionMismatch
	}
	return nil
}

func currentTokenVersion(ctx context.Context, users UserTokenVersionStore, sessions AuthSessionStore, userID string) (int64, error) {
	// access token 中间件对延迟敏感，因此先查 Redis，再回退到仓储。
	currentVersion, err := sessions.GetCachedTokenVersion(ctx, userID)
	if err == nil {
		return currentVersion, nil
	}
	if !errors.Is(err, authdomain.ErrTokenVersionCacheMiss) {
		return 0, err
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		// token 用户 ID 是外部 UUID，解析可保护仓储调用不接收畸形 claims。
		return 0, fmt.Errorf("parse user id: %w", err)
	}
	currentVersion, err = users.GetTokenVersion(ctx, parsedUserID)
	if err != nil {
		return 0, err
	}
	if err := sessions.CacheTokenVersion(ctx, userID, currentVersion); err != nil {
		return 0, err
	}
	return currentVersion, nil
}

// RevokeAllUserSessions 递增 token version、刷新缓存并删除全部 refresh 会话。
func (m *authSessionLifecycle) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*authdomain.SessionRevocationResult, error) {
	tokenVersion, err := m.users.IncrementTokenVersion(ctx, userID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "revoke all user sessions user not found", zap.String("user_id", userID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "increment token version failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := m.sessions.CacheTokenVersion(ctx, userID.String(), tokenVersion); err != nil {
		// 删除会话前先刷新缓存，使 access token 中间件立即拒绝旧 token。
		logger.Error(ctx, "refresh token version cache failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := m.sessions.DeleteAllUserSessions(ctx, userID.String()); err != nil {
		logger.Error(ctx, "delete all user sessions failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return &authdomain.SessionRevocationResult{UserID: userID, TokenVersion: tokenVersion}, nil
}
