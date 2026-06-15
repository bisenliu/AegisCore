package sessions

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

// RevokeAllUserSessions 递增 token version、刷新缓存并删除全部 refresh 会话。
func (m *lifecycle) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*authdomain.SessionRevocationResult, error) {
	tokenVersion, err := m.users.IncrementTokenVersion(ctx, userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			logger.Warn(ctx, "revoke all user sessions user not found", zap.String("user_id", userID.String()))
			return nil, identity.ErrUserNotFound
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
func (m *lifecycle) RevokeUserSessionsAtVersion(ctx context.Context, userID uuid.UUID, tokenVersion int64) error {
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
