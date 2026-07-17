package auth

import (
	"context"
	"fmt"
	"sync"

	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/localcache"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	commonvalidation "github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-service/ent"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
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

type credentialStoreParams struct {
	fx.In

	Client *ent.Client `name:"primary_db"`
}

type sessionStoreParams struct {
	fx.In

	Redis     *rediscache.Client `name:"cache_redis"`
	Config    *serviceconfig.Config
	PurgePool authredis.PurgeTaskPool `name:"auth_session_purge_pool"`
	Metrics   authapplication.Metrics
}

type sessionPurgePoolParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	// Redis 只在 Fx module 边界表达关闭顺序：auth 自有 pool 必须先于共享 Redis client 停止。
	Redis *rediscache.Client `name:"cache_redis"`
	Log   *zap.Logger
}

type sessionPurgePoolResult struct {
	fx.Out

	Pool authredis.PurgeTaskPool `name:"auth_session_purge_pool"`
}

type tokenVersionCacheParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *serviceconfig.Config
	Users     authapplication.UserTokenVersionStore
	Cache     authapplication.TokenVersionCache
}

type tokenVersionCacheResult struct {
	fx.Out

	Cache authvalidators.LocalTokenVersionCache `name:"auth_token_version_cache"`
	Stats localcache.StatsSource                `name:"auth_token_version_cache"`
}

type tokenVersionLocalCacheResource struct {
	cache authvalidators.LocalTokenVersionCache
	stats localcache.StatsSource

	closeOnce sync.Once
	close     func()
}

type tokenVersionValidatorParams struct {
	fx.In

	Cache   authvalidators.LocalTokenVersionCache `name:"auth_token_version_cache"`
	Metrics authapplication.Metrics
}

type tokenVersionValidatorResult struct {
	fx.Out

	Validator   commonauth.TokenVersionValidator
	Invalidator authvalidators.TokenVersionLocalInvalidator
}

type authControllerParams struct {
	fx.In

	Login          authcommand.LoginUseCase
	Refresh        authcommand.RefreshTokenUseCase
	ChangePassword authcommand.ChangePasswordUseCase
	LogoutCurrent  authcommand.LogoutCurrentSessionUseCase
	LogoutAll      authcommand.LogoutAllSessionsUseCase
	Validator      *commonvalidation.Validator
}

// Module 组装认证功能的应用服务、HTTP 传输层和基础设施适配器。
var Module = fx.Module("feature-auth",
	fx.Provide(
		// Fx 分类：横切能力 - auth feature 指标。
		newAuthMetrics,
		// Fx 分类：Feature 基础设施 - PostgreSQL 与 Redis port adapter。
		fx.Annotate(
			newCredentialStore,
			fx.As(new(authapplication.UserTokenVersionStore)),
			fx.As(new(authapplication.UserCredentialStore)),
		),
		fx.Annotate(
			newSessionStore,
			fx.As(new(authapplication.TokenVersionCache)),
			fx.As(new(authapplication.RefreshSessionStore)),
			fx.As(new(authapplication.PasswordChangeSessionStore)),
		),
		// Fx 分类：资源 - auth feature 私有清理 worker pool。
		newSessionPurgePool,
		// Fx 分类：资源 - token version 本地缓存及其生命周期。
		newTokenVersionLocalCache,
		// Fx 分类：Feature 应用 - token、凭据和会话安全能力。
		newTokenVersionValidator,
		fx.Annotate(
			authcredentials.NewVerifier,
			fx.From(new(authapplication.UserCredentialStore), new(*password.Service)),
		),
		authtokens.NewIssuer,
		authtokens.NewAccessTokenVerifier,
		newAuthSessionLifecycle,
		newRefreshTokenSettings,
		// Fx 分类：Feature 应用 - 认证命令用例。
		authcommand.NewLoginUseCase,
		authcommand.NewRefreshTokenUseCase,
		authcommand.NewChangePasswordUseCase,
		authcommand.NewLogoutCurrentSessionUseCase,
		authcommand.NewLogoutAllSessionsUseCase,
		// Fx 分类：传输 - auth HTTP controller。
		newAuthController,
	),
)

func newCredentialStore(params credentialStoreParams) *authpostgres.CredentialStore {
	return authpostgres.NewCredentialStore(params.Client)
}

func newSessionStore(params sessionStoreParams) (*authredis.SessionStore, error) {
	keys, err := authredis.NewKeyCatalog(params.Config.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new auth redis keys: %w", err)
	}
	return authredis.NewSessionStore(authredis.SessionStoreOptions{
		Redis:                params.Redis,
		Keys:                 keys,
		TokenVersionCacheTTL: params.Config.Auth.TokenVersionCacheTTL,
		PurgePool:            params.PurgePool,
		Metrics:              params.Metrics,
	}), nil
}

func newSessionPurgePool(params sessionPurgePoolParams) (sessionPurgePoolResult, error) {
	pool, err := authredis.NewSessionPurgePool(params.Log)
	if err != nil {
		return sessionPurgePoolResult{}, err
	}
	params.Lifecycle.Append(fx.Hook{OnStop: pool.Stop})
	return sessionPurgePoolResult{Pool: pool}, nil
}

func newAuthController(params authControllerParams) *authhttp.AuthController {
	return authhttp.NewAuthController(authhttp.AuthControllerOptions{
		Login:          params.Login,
		Refresh:        params.Refresh,
		ChangePassword: params.ChangePassword,
		LogoutCurrent:  params.LogoutCurrent,
		LogoutAll:      params.LogoutAll,
		Validator:      params.Validator,
	})
}

func newAuthSessionLifecycle(users authapplication.UserTokenVersionStore, tokenVersionCache authapplication.TokenVersionCache, sessions authapplication.RefreshSessionStore, passwordChangeSessions authapplication.PasswordChangeSessionStore, tokenVersions authvalidators.TokenVersionLocalInvalidator, cfg *serviceconfig.Config) authsessions.Lifecycle {
	return authsessions.NewLifecycle(users, tokenVersionCache, sessions, passwordChangeSessions, cfg.Auth.MaxActiveSessionsPerUser, tokenVersions)
}

func newRefreshTokenSettings(cfg *serviceconfig.Config) authcommand.RefreshTokenSettings {
	return authcommand.RefreshTokenSettings{RefreshTokenRotation: cfg.Auth.RefreshTokenRotation}
}

func newTokenVersionLocalCache(params tokenVersionCacheParams) (tokenVersionCacheResult, error) {
	resource, err := newTokenVersionLocalCacheResource(params.Config.Auth.TokenVersionCache, params.Users, params.Cache)
	if err != nil {
		return tokenVersionCacheResult{}, err
	}
	if params.Lifecycle != nil {
		params.Lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
			return resource.Close()
		}})
	}
	return tokenVersionCacheResult{Cache: resource.cache, Stats: resource.stats}, nil
}

func newTokenVersionLocalCacheResource(cfg serviceconfig.FeatureCacheConfig, users authapplication.UserTokenVersionStore, cache authapplication.TokenVersionCache) (*tokenVersionLocalCacheResource, error) {
	if !cfg.IsEnabled() {
		direct := authvalidators.NewDirectTokenVersionCache(users, cache)
		return &tokenVersionLocalCacheResource{cache: direct, stats: direct}, nil
	}
	local, err := localcache.New[string, int64](localcache.Config[string]{
		Name:        authTokenVersionCacheName,
		Capacity:    cfg.SizeValue(),
		TTL:         cfg.TTLValue(),
		LoadTimeout: cfg.LoadTimeoutValue(),
		KeyString:   func(key string) string { return key },
	}, func(ctx context.Context, userID string) (int64, error) {
		return authvalidators.Current(ctx, users, cache, userID)
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("create auth token version localcache: %w", err)
	}
	return &tokenVersionLocalCacheResource{cache: local, stats: local, close: local.Close}, nil
}

func (r *tokenVersionLocalCacheResource) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.close != nil {
			r.close()
		}
	})
	return nil
}

func newTokenVersionValidator(params tokenVersionValidatorParams) tokenVersionValidatorResult {
	validator := authvalidators.NewCachingValidator(params.Cache)
	return tokenVersionValidatorResult{Validator: authvalidators.NewMetricsTokenVersionValidator(validator, params.Metrics), Invalidator: validator}
}
