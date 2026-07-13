package redis

import (
	"context"
	"errors"
	"fmt"

	rediscache "github.com/redis/go-redis/v9"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

const (
	cacheTokenVersionResultStored  = 1
	cacheTokenVersionResultSkipped = 2
)

// GetCachedTokenVersion 返回缓存的 token version，未命中时返回 ErrTokenVersionCacheMiss。
func (r *SessionStore) GetCachedTokenVersion(ctx context.Context, userID string) (int64, error) {
	key := r.tokenVersionKey(userID)
	value, err := r.redis.Get(ctx, key).Result()
	if errors.Is(err, rediscache.Nil) {
		return 0, authdomain.ErrTokenVersionCacheMiss
	}
	if err != nil {
		return 0, fmt.Errorf("get token version cache: %w", err)
	}
	version, err := parseTokenVersion(value)
	if err != nil || version <= 0 {
		return 0, authdomain.ErrTokenVersionCacheMiss
	}
	return version, nil
}

// CacheTokenVersion 存储用户 token version，供中间件执行撤销校验。
func (r *SessionStore) CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	ttl := r.tokenVersionCacheTTL
	if ttl <= 0 {
		// 非正数配置表示使用有界默认过期窗口，而不是创建永久缓存项。
		ttl = defaultTokenVersionCacheTTL
	}
	result, err := cacheTokenVersionScript.Run(ctx, r.redis, []string{r.tokenVersionKey(userID)}, formatTokenVersion(tokenVersion), milliseconds(ttl)).Int64()
	if err != nil {
		return fmt.Errorf("set token version cache: %w", err)
	}
	if result != cacheTokenVersionResultStored && result != cacheTokenVersionResultSkipped {
		return fmt.Errorf("set token version cache: unexpected script result %d", result)
	}
	return nil
}

// DeleteCachedTokenVersion 删除用户 token version 缓存，使后续校验回源 PostgreSQL。
func (r *SessionStore) DeleteCachedTokenVersion(ctx context.Context, userID string) error {
	if err := r.redis.Del(ctx, r.tokenVersionKey(userID)).Err(); err != nil {
		return fmt.Errorf("delete token version cache: %w", err)
	}
	return nil
}
