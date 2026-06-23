package auth

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
)

const authTokenVersionCacheName = "auth_token_version" // #nosec G101 -- 本地缓存名称，不包含真实凭据。

type tokenVersionCacheParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Users     authapplication.UserTokenVersionStore
	Sessions  authapplication.AuthSessionStore
}

type tokenVersionCacheResult struct {
	fx.Out

	Cache *localcache.Cache[string, int64] `name:"auth_token_version_cache"`
	Stats localcache.StatsSource           `name:"auth_token_version_cache"`
}

func newTokenVersionLocalCache(params tokenVersionCacheParams) (tokenVersionCacheResult, error) {
	cfg, ok := params.Config.LocalCache.Instance(authTokenVersionCacheName)
	if !ok {
		return tokenVersionCacheResult{}, fmt.Errorf("local_cache.%s is required", authTokenVersionCacheName)
	}
	cache, err := localcache.New[string, int64](localcache.Config[string]{
		Name:        authTokenVersionCacheName,
		Capacity:    cfg.Capacity,
		TTL:         cfg.TTL,
		LoadTimeout: cfg.LoadTimeout,
		KeyString:   func(key string) string { return key },
		NumCounters: cfg.NumCounters,
		BufferItems: cfg.BufferItems,
	}, func(ctx context.Context, userID string) (int64, error) {
		return authvalidators.Current(ctx, params.Users, params.Sessions, userID)
	}, nil)
	if err != nil {
		return tokenVersionCacheResult{}, fmt.Errorf("create auth token version localcache: %w", err)
	}

	params.Lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		cache.Close()
		return nil
	}})
	return tokenVersionCacheResult{Cache: cache, Stats: cache}, nil
}
