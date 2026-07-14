package validators

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// TokenVersionValidator 使用 feature 注入的本地读取策略校验 token version。
type TokenVersionValidator struct {
	cache LocalTokenVersionCache
}

// LocalTokenVersionCache 定义 token version 本地读取与失效所需的最小接口。
type LocalTokenVersionCache interface {
	GetOrLoad(ctx context.Context, userID string) (int64, error)
	Delete(userID string) error
}

// DirectTokenVersionCache 在关闭本地缓存时逐次回源，并保留稳定的指标来源。
type DirectTokenVersionCache struct {
	users     authapplication.UserTokenVersionStore
	cache     authapplication.TokenVersionCache
	load      atomic.Uint64
	loadError atomic.Uint64
}

// TokenVersionLocalInvalidator 失效本实例内 token version 本地缓存。
type TokenVersionLocalInvalidator interface {
	InvalidateTokenVersion(userID string) error
}

// NewCachingValidator 构造使用外部注入 localcache 的 token version 校验器。
func NewCachingValidator(cache LocalTokenVersionCache) *TokenVersionValidator {
	return &TokenVersionValidator{cache: cache}
}

// NewDirectTokenVersionCache 构造不保留进程内状态的 token version 回源器。
func NewDirectTokenVersionCache(users authapplication.UserTokenVersionStore, cache authapplication.TokenVersionCache) *DirectTokenVersionCache {
	return &DirectTokenVersionCache{users: users, cache: cache}
}

// GetOrLoad 每次通过 Redis 投影或用户存储读取当前 token version。
func (c *DirectTokenVersionCache) GetOrLoad(ctx context.Context, userID string) (int64, error) {
	c.load.Add(1)
	version, err := Current(ctx, c.users, c.cache, userID)
	if err != nil {
		c.loadError.Add(1)
	}
	return version, err
}

// Delete 在无本地缓存模式下是安全 no-op。
func (c *DirectTokenVersionCache) Delete(string) error {
	return nil
}

// Name 返回供 metrics 使用的稳定缓存实例名。
func (c *DirectTokenVersionCache) Name() string {
	return "auth_token_version"
}

// Stats 返回无本地缓存模式下的回源统计。
func (c *DirectTokenVersionCache) Stats() localcache.Stats {
	loads := c.load.Load()
	return localcache.Stats{Miss: loads, Load: loads, LoadError: c.loadError.Load()}
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
