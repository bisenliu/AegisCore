package tokenversion

import (
	"context"
	"errors"
	"fmt"

	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type tokenVersionValidator struct {
	users    authapplication.UserTokenVersionStore
	sessions authapplication.AuthSessionStore
}

// NewValidator 构造由缓存和持久化存储支撑的 token version 校验器。
func NewValidator(users authapplication.UserTokenVersionStore, sessions authapplication.AuthSessionStore) commonauth.TokenVersionValidator {
	return &tokenVersionValidator{users: users, sessions: sessions}
}

// ValidateTokenVersion 拒绝 version 不再匹配当前用户版本的 token。
func (v *tokenVersionValidator) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	currentVersion, err := Current(ctx, v.users, v.sessions, userID)
	if err != nil {
		return err
	}
	return commonauth.ValidateTokenVersion(tokenVersion, currentVersion)
}

// Current 使用 Redis token version cache，并在 miss 时回源用户凭据存储。
func Current(ctx context.Context, users authapplication.UserTokenVersionStore, sessions authapplication.AuthSessionStore, userID string) (int64, error) {
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
