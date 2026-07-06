package auth

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authpostgres "github.com/aegiscore/user-service/internal/features/auth/infrastructure/postgres"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
)

const authTokenVersionCacheName = "auth_token_version" // #nosec G101 -- 本地缓存名称，不包含真实凭据。

type tokenVersionCacheParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Users     authapplication.UserTokenVersionStore
	Cache     authapplication.TokenVersionCache
}

type tokenVersionCacheResult struct {
	fx.Out

	Cache *localcache.Cache[string, int64] `name:"auth_token_version_cache"`
	Stats localcache.StatsSource           `name:"auth_token_version_cache"`
}

type tokenVersionValidatorParams struct {
	fx.In

	Cache   *localcache.Cache[string, int64] `name:"auth_token_version_cache"`
	Metrics authapplication.Metrics
}

type tokenVersionValidatorResult struct {
	fx.Out

	Validator   commonauth.TokenVersionValidator
	Invalidator authvalidators.TokenVersionLocalInvalidator
}

// Module 组装认证功能的应用服务、HTTP 传输层和基础设施适配器。
var Module = fx.Module("feature-auth",
	fx.Provide(
		newAuthMetrics,
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapplication.UserCredentialStore)),
		),
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapplication.UserTokenVersionStore)),
		),
		fx.Annotate(
			authredis.NewSessionStore,
			fx.As(new(authapplication.TokenVersionCache)),
			fx.As(new(authapplication.RefreshSessionStore)),
			fx.As(new(authapplication.PasswordChangeSessionStore)),
		),
		fx.Annotate(
			authredis.NewSessionPurgePool,
			fx.As(new(authredis.PurgeTaskPool)),
			fx.ResultTags(`name:"auth_session_purge_pool"`),
		),
		newTokenVersionLocalCache,
		newTokenVersionValidator,
		authcredentials.NewVerifier,
		authtokens.NewIssuer,
		newAuthSessionLifecycle,
		authcommand.NewLoginUseCase,
		authcommand.NewRefreshTokenUseCase,
		authcommand.NewChangePasswordUseCase,
		authcommand.NewLogoutCurrentSessionUseCase,
		authcommand.NewLogoutAllSessionsUseCase,
		authhttp.NewAuthController,
	),
)

func newAuthSessionLifecycle(users authapplication.UserTokenVersionStore, tokenVersionCache authapplication.TokenVersionCache, sessions authapplication.RefreshSessionStore, passwordChangeSessions authapplication.PasswordChangeSessionStore, tokenVersions authvalidators.TokenVersionLocalInvalidator, cfg *config.Config) authsessions.Lifecycle {
	return authsessions.NewLifecycle(users, tokenVersionCache, sessions, passwordChangeSessions, cfg.Auth.MaxActiveSessionsPerUser, tokenVersions)
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
		return authvalidators.Current(ctx, params.Users, params.Cache, userID)
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

func newTokenVersionValidator(params tokenVersionValidatorParams) tokenVersionValidatorResult {
	validator := authvalidators.NewCachingValidator(params.Cache)
	return tokenVersionValidatorResult{Validator: authvalidators.NewMetricsTokenVersionValidator(validator, params.Metrics), Invalidator: validator}
}
