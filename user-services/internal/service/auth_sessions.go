package service

import (
	"context"
	"errors"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/errmsg"
	"github.com/aegiscore/user-services/internal/repository"
	"go.uber.org/zap"
)

type AuthSessionManager interface {
	CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error
	ValidatePasswordChangeClaims(ctx context.Context, claims *auth.Claims) error
	ValidateRefreshSession(ctx context.Context, claims *auth.Claims) (repository.AuthSession, int64, error)
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	DeleteAllUserSessions(ctx context.Context, userID string) error
	InvalidateUserTokenVersion(ctx context.Context, userID string) error
}

type authSessionManager struct {
	sessions repository.AuthSessionRepository
}

func newAuthSessionManager(sessions repository.AuthSessionRepository) AuthSessionManager {
	return &authSessionManager{sessions: sessions}
}

func (m *authSessionManager) CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error {
	if err := m.sessions.CreateSession(ctx, repository.AuthSession{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		logger.Error(ctx, "create auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return response.FromError(err)
	}
	return nil
}

func (m *authSessionManager) ValidatePasswordChangeClaims(ctx context.Context, claims *auth.Claims) error {
	currentVersion, err := m.sessions.GetCurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "password change user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "get password change token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return response.FromError(err)
	}
	if currentVersion != claims.TokenVersion {
		logger.Warn(ctx, "password change token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("token_version", claims.TokenVersion))
		return response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	return nil
}

func (m *authSessionManager) ValidateRefreshSession(ctx context.Context, claims *auth.Claims) (repository.AuthSession, int64, error) {
	session, err := m.sessions.GetSession(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrAuthSessionNotFound) {
			logger.Warn(ctx, "refresh session not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return repository.AuthSession{}, 0, response.TokenInvalidError(errmsg.MsgMissingSession)
		}
		logger.Error(ctx, "get refresh session failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return repository.AuthSession{}, 0, response.FromError(err)
	}
	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		logger.Warn(ctx, "refresh session mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("session_token_version", session.TokenVersion), zap.Int64("token_version", claims.TokenVersion))
		return repository.AuthSession{}, 0, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	currentVersion, err := m.sessions.GetCurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "refresh user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return repository.AuthSession{}, 0, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "get refresh token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return repository.AuthSession{}, 0, response.FromError(err)
	}
	if currentVersion != session.TokenVersion {
		logger.Warn(ctx, "refresh token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("session_token_version", session.TokenVersion))
		return repository.AuthSession{}, 0, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	return session, currentVersion, nil
}

func (m *authSessionManager) DeleteSession(ctx context.Context, userID string, sessionID string) error {
	if err := m.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		logger.Error(ctx, "delete auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Error(err))...)
		return response.FromError(err)
	}
	return nil
}

func (m *authSessionManager) DeleteAllUserSessions(ctx context.Context, userID string) error {
	if err := m.sessions.DeleteAllUserSessions(ctx, userID); err != nil {
		logger.Error(ctx, "delete all user sessions failed", logger.StackTrace(zap.String("user_id", userID), zap.Error(err))...)
		return response.FromError(err)
	}
	return nil
}

func (m *authSessionManager) InvalidateUserTokenVersion(ctx context.Context, userID string) error {
	if err := m.sessions.InvalidateUserTokenVersion(ctx, userID); err != nil {
		logger.Error(ctx, "invalidate token version failed", logger.StackTrace(zap.String("user_id", userID), zap.Error(err))...)
		return response.FromError(err)
	}
	return nil
}
