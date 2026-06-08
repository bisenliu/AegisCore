package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuthSessionManager interface {
	CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error
	ValidatePasswordChangeClaims(ctx context.Context, claims *auth.Claims) error
	ValidateRefreshSession(ctx context.Context, claims *auth.Claims) (repository.AuthSession, int64, error)
	DeleteSession(ctx context.Context, userID string, sessionID string) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*SessionRevocationResult, error)
}

type TokenVersionValidator interface {
	ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
}

type SessionRevocationResult struct {
	UserID       uuid.UUID
	TokenVersion int64
}

type authSessionManager struct {
	users    repository.UserTokenVersionRepository
	sessions repository.AuthSessionRepository
}

type tokenVersionValidator struct {
	users    repository.UserTokenVersionRepository
	sessions repository.AuthSessionRepository
}

func NewTokenVersionValidator(users repository.UserTokenVersionRepository, sessions repository.AuthSessionRepository) TokenVersionValidator {
	return &tokenVersionValidator{users: users, sessions: sessions}
}

func newAuthSessionManager(users repository.UserTokenVersionRepository, sessions repository.AuthSessionRepository) AuthSessionManager {
	return &authSessionManager{users: users, sessions: sessions}
}

func (m *authSessionManager) CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error {
	if err := m.sessions.CreateSession(ctx, repository.AuthSession{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		logger.Error(ctx, "create auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return response.FromError(err)
	}
	return nil
}

func (m *authSessionManager) ValidatePasswordChangeClaims(ctx context.Context, claims *auth.Claims) error {
	currentVersion, err := m.currentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
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

func (m *authSessionManager) ValidateRefreshSession(ctx context.Context, claims *auth.Claims) (repository.AuthSession, int64, error) {
	session, err := m.sessions.GetSession(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrAuthSessionNotFound) {
			logger.Warn(ctx, "refresh session not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return repository.AuthSession{}, 0, response.TokenInvalidError(messages.MissingSession)
		}
		logger.Error(ctx, "get refresh session failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return repository.AuthSession{}, 0, response.FromError(err)
	}
	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		logger.Warn(ctx, "refresh session mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("session_token_version", session.TokenVersion), zap.Int64("token_version", claims.TokenVersion))
		return repository.AuthSession{}, 0, response.TokenInvalidError(messages.MissingSession)
	}
	currentVersion, err := m.currentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "refresh user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return repository.AuthSession{}, 0, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "get refresh token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return repository.AuthSession{}, 0, response.FromError(err)
	}
	if currentVersion != session.TokenVersion {
		logger.Warn(ctx, "refresh token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("session_token_version", session.TokenVersion))
		return repository.AuthSession{}, 0, response.TokenInvalidError(messages.MissingSession)
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

func (m *authSessionManager) currentTokenVersion(ctx context.Context, userID string) (int64, error) {
	return currentTokenVersion(ctx, m.users, m.sessions, userID)
}

func (v *tokenVersionValidator) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	currentVersion, err := currentTokenVersion(ctx, v.users, v.sessions, userID)
	if err != nil {
		return err
	}
	if currentVersion != tokenVersion {
		return repository.ErrTokenVersionMismatch
	}
	return nil
}

func currentTokenVersion(ctx context.Context, users repository.UserTokenVersionRepository, sessions repository.AuthSessionRepository, userID string) (int64, error) {
	currentVersion, err := sessions.GetCachedTokenVersion(ctx, userID)
	if err == nil {
		return currentVersion, nil
	}
	if !errors.Is(err, repository.ErrTokenVersionCacheMiss) {
		return 0, err
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
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

func (m *authSessionManager) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*SessionRevocationResult, error) {
	tokenVersion, err := m.users.IncrementTokenVersion(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "revoke all user sessions user not found", zap.String("user_id", userID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "increment token version failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := m.sessions.InvalidateUserTokenVersion(ctx, userID.String()); err != nil {
		logger.Error(ctx, "invalidate token version failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := m.sessions.DeleteAllUserSessions(ctx, userID.String()); err != nil {
		logger.Error(ctx, "delete all user sessions failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return &SessionRevocationResult{UserID: userID, TokenVersion: tokenVersion}, nil
}
