package validators

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// TokenVersionValidator 使用 bounded localcache 校验 token version。
type TokenVersionValidator struct {
	cache *localcache.Cache[string, int64]
}

// TokenVersionLocalInvalidator 失效本实例内 token version 本地缓存。
type TokenVersionLocalInvalidator interface {
	InvalidateTokenVersion(userID string) error
}

// NewCachingValidator 构造使用外部注入 localcache 的 token version 校验器。
func NewCachingValidator(cache *localcache.Cache[string, int64]) *TokenVersionValidator {
	return &TokenVersionValidator{cache: cache}
}

// ValidateTokenVersion 拒绝 version 不再匹配当前用户版本的 token。
func (v *TokenVersionValidator) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	currentVersion, err := v.Current(ctx, userID)
	if err != nil {
		return err
	}
	return commonauth.ValidateTokenVersion(tokenVersion, currentVersion)
}

// Current 返回本实例缓存或后端存储中的当前 token version。
func (v *TokenVersionValidator) Current(ctx context.Context, userID string) (int64, error) {
	return v.cache.GetOrLoad(ctx, userID)
}

// InvalidateTokenVersion 删除本实例内指定用户的 token version 本地缓存。
func (v *TokenVersionValidator) InvalidateTokenVersion(userID string) error {
	if err := v.cache.Delete(userID); err != nil {
		if errors.Is(err, localcache.ErrClosed) {
			return nil
		}
		return fmt.Errorf("delete local token version cache: %w", err)
	}
	return nil
}

// Current 使用 Redis token version cache，并在 miss 时回源用户凭据存储。
func Current(ctx context.Context, users authapplication.UserTokenVersionStore, cache authapplication.TokenVersionCache, userID string) (int64, error) {
	// access token 中间件对延迟敏感，因此先查 Redis，再回退到仓储。
	currentVersion, err := cache.GetCachedTokenVersion(ctx, userID)
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
	if err := cache.CacheTokenVersion(ctx, userID, currentVersion); err != nil {
		logger.Error(ctx, "backfill token version cache failed", logger.StackTrace(zap.String("user_id", userID), zap.Int64("token_version", currentVersion), zap.Error(err))...)
		return 0, fmt.Errorf("backfill token version cache: %w", err)
	}
	return currentVersion, nil
}
