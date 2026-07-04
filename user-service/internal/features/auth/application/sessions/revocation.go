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

// RevokeAllUserSessions 递增 token version，并尽力刷新 Redis 投影与删除全部 refresh 会话。
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
	projectionErr := m.RevokeUserSessionsAtVersion(ctx, userID, tokenVersion)
	if projectionErr != nil {
		logger.Error(ctx, "revoke all user sessions projection failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Int64("token_version", tokenVersion), zap.Error(projectionErr))...)
	}
	logger.Info(ctx, "all user sessions revoked", zap.String("user_id", userID.String()), zap.Int64("token_version", tokenVersion))
	return &authdomain.SessionRevocationResult{UserID: userID, TokenVersion: tokenVersion, ProjectionError: projectionErr}, nil
}

// RevokeUserSessionsAtVersion 刷新 token version 投影并删除全部 refresh 会话，不修改 PostgreSQL 版本。
func (m *lifecycle) RevokeUserSessionsAtVersion(ctx context.Context, userID uuid.UUID, tokenVersion int64) error {
	userIDString := userID.String()
	var projectionErr error
	// 先清理本实例已有的旧 token version，降低撤销开始后继续命中本地旧值的窗口。
	if err := m.invalidateLocalTokenVersion(ctx, userIDString); err != nil {
		projectionErr = errors.Join(projectionErr, fmt.Errorf("invalidate local token version cache before projection: %w", err))
	}
	if err := m.tokenVersions.CacheTokenVersion(ctx, userIDString, tokenVersion); err != nil {
		logger.Error(ctx, "refresh token version cache failed", logger.StackTrace(zap.String("user_id", userIDString), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		projectionErr = errors.Join(projectionErr, fmt.Errorf("refresh token version cache: %w", err))
		if evictErr := m.tokenVersions.DeleteCachedTokenVersion(ctx, userIDString); evictErr != nil {
			logger.Error(ctx, "delete token version cache failed", logger.StackTrace(zap.String("user_id", userIDString), zap.Int64("token_version", tokenVersion), zap.Error(evictErr))...)
			projectionErr = errors.Join(projectionErr, fmt.Errorf("delete token version cache: %w", evictErr))
		}
	}
	// Redis 投影刷新期间并发校验可能重新加载旧值；刷新后再清一次，确保后续 miss 回源新投影或数据库主事实。
	if err := m.invalidateLocalTokenVersion(ctx, userIDString); err != nil {
		projectionErr = errors.Join(projectionErr, fmt.Errorf("invalidate local token version cache after projection: %w", err))
	}
	if err := m.sessions.DeleteAllUserSessions(ctx, userIDString); err != nil {
		logger.Error(ctx, "delete all user sessions failed", logger.StackTrace(zap.String("user_id", userIDString), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		projectionErr = errors.Join(projectionErr, fmt.Errorf("delete all user sessions: %w", err))
	}
	// refresh session 删除不改变 token version；这里作为撤销流程结束前的最终兜底，清掉过程中可能产生的本地值。
	if err := m.invalidateLocalTokenVersion(ctx, userIDString); err != nil {
		projectionErr = errors.Join(projectionErr, fmt.Errorf("invalidate local token version cache after session deletion: %w", err))
	}
	return projectionErr
}
