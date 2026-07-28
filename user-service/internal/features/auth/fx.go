package auth

import (
	"context"
	"fmt"
	"sync"

	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/workerpool"
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

// Fx 模块与选项

// Module 组装认证功能的应用服务、HTTP 传输层和基础设施适配器。
var Module = fx.Module(
	"feature-auth",
	authMetricsOptions,
	authInfrastructureOptions,
	authResourceOptions,
	authApplicationOptions,
	authTransportOptions,
)

var authMetricsOptions = fx.Options(
	fx.Provide(
		newAuthMetrics,
	),
)

var authInfrastructureOptions = fx.Options(
	fx.Provide(
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
	),
)

var authResourceOptions = fx.Options(
	fx.Provide(
		newSessionPurgePool,
		newTokenVersionLocalCache,
	),
)

var authApplicationOptions = fx.Options(
	fx.Provide(
		newTokenVersionValidator,
		fx.Annotate(
			authcredentials.NewVerifier,
			fx.From(new(authapplication.UserCredentialStore), new(*password.Service)),
		),
		authtokens.NewIssuer,
		authtokens.NewAccessTokenVerifier,
		newAuthSessionLifecycle,
		newRefreshTokenSettings,
		authcommand.NewLoginUseCase,
		authcommand.NewRefreshTokenUseCase,
		authcommand.NewChangePasswordUseCase,
		authcommand.NewLogoutCurrentSessionUseCase,
		authcommand.NewLogoutAllSessionsUseCase,
	),
)

var authTransportOptions = fx.Options(
	fx.Provide(
		newAuthController,
	),
)

// Fx 参数与结果：基础设施

type CredentialStoreParams struct {
	fx.In

	Client *ent.Client `name:"primary_db"`
}

type SessionStoreParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Redis     rediscache.UniversalClient `name:"cache_redis"`
	Settings  serviceconfig.AuthSettings
	PurgePool authredis.PurgeTaskPool `name:"auth_session_purge_pool"`
	Metrics   authapplication.Metrics
}

// Fx 参数与结果：运行时资源

type SessionPurgePoolParams struct {
	fx.In

	Log *zap.Logger
}

type SessionPurgePoolResult struct {
	fx.Out

	Pool authredis.PurgeTaskPool `name:"auth_session_purge_pool"`
}

type TokenVersionLocalCacheParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Settings  serviceconfig.AuthSettings
	Users     authapplication.UserTokenVersionStore
	Cache     authapplication.TokenVersionCache
}

type TokenVersionLocalCacheResult struct {
	fx.Out

	Cache authvalidators.LocalTokenVersionCache `name:"auth_token_version_cache"`
	Stats localcache.StatsSource                `name:"auth_token_version_cache"`
}

// Fx 参数与结果：应用服务

type TokenVersionValidatorParams struct {
	fx.In

	Cache   authvalidators.LocalTokenVersionCache `name:"auth_token_version_cache"`
	Metrics authapplication.Metrics
}

type TokenVersionValidatorResult struct {
	fx.Out

	Validator   commonauth.TokenVersionValidator
	Invalidator authvalidators.TokenVersionLocalInvalidator
}

// Fx 参数与结果：传输层

type AuthControllerParams struct {
	fx.In

	Login          authcommand.LoginUseCase
	Refresh        authcommand.RefreshTokenUseCase
	ChangePassword authcommand.ChangePasswordUseCase
	LogoutCurrent  authcommand.LogoutCurrentSessionUseCase
	LogoutAll      authcommand.LogoutAllSessionsUseCase
	Validator      *commonvalidation.Validator
}

// 运行时资源 holder

type tokenVersionCacheResource struct {
	cache authvalidators.LocalTokenVersionCache
	stats localcache.StatsSource

	closeOnce sync.Once
	close     func()
}

type sessionPurgePoolHolder struct {
	mu   sync.RWMutex
	log  *zap.Logger
	pool *workerpool.Pool
}

type tokenVersionCacheHolder struct {
	mu       sync.RWMutex
	cfg      serviceconfig.FeatureCacheConfig
	users    authapplication.UserTokenVersionStore
	cache    authapplication.TokenVersionCache
	resource *tokenVersionCacheResource
}

// Provider：基础设施

func newCredentialStore(params CredentialStoreParams) *authpostgres.CredentialStore {
	return authpostgres.NewCredentialStore(params.Client)
}

func newSessionStore(params SessionStoreParams) (*authredis.SessionStore, error) {
	keys, err := authredis.NewKeyCatalog(params.Settings.AppName)
	if err != nil {
		return nil, fmt.Errorf("new auth redis keys: %w", err)
	}
	store := authredis.NewSessionStore(authredis.SessionStoreOptions{
		Redis:                params.Redis,
		Keys:                 keys,
		TokenVersionCacheTTL: params.Settings.TokenVersionCacheTTL,
		PurgePool:            params.PurgePool,
		Metrics:              params.Metrics,
	})
	if starter, ok := params.PurgePool.(*sessionPurgePoolHolder); ok && params.Lifecycle != nil {
		params.Lifecycle.Append(fx.Hook{
			OnStart: starter.Start,
			OnStop:  starter.Stop,
		})
	}
	return store, nil
}

// Provider：运行时资源

func newSessionPurgePool(params SessionPurgePoolParams) (SessionPurgePoolResult, error) {
	return SessionPurgePoolResult{Pool: &sessionPurgePoolHolder{log: params.Log}}, nil
}

func newTokenVersionLocalCache(params TokenVersionLocalCacheParams) (TokenVersionLocalCacheResult, error) {
	resource := &tokenVersionCacheHolder{cfg: params.Settings.TokenVersionCache, users: params.Users, cache: params.Cache}
	if params.Lifecycle != nil {
		params.Lifecycle.Append(fx.Hook{OnStart: resource.Start, OnStop: resource.Close})
	} else if err := resource.Start(context.Background()); err != nil {
		return TokenVersionLocalCacheResult{}, err
	}
	return TokenVersionLocalCacheResult{Cache: resource, Stats: resource}, nil
}

func newTokenVersionLocalCacheResource(cfg serviceconfig.FeatureCacheConfig, users authapplication.UserTokenVersionStore, cache authapplication.TokenVersionCache) (*tokenVersionCacheResource, error) {
	if !cfg.Enabled {
		direct := authvalidators.NewDirectTokenVersionCache(users, cache)
		return &tokenVersionCacheResource{cache: direct, stats: direct}, nil
	}
	local, err := localcache.NewLoadingCache[string, int64](cfg.Localcache(authTokenVersionCacheName), func(ctx context.Context, userID string) (int64, error) {
		return authvalidators.Current(ctx, users, cache, userID)
	})
	if err != nil {
		return nil, fmt.Errorf("create auth token version localcache: %w", err)
	}
	return &tokenVersionCacheResource{cache: local, stats: local, close: local.Close}, nil
}

// Provider：应用服务

func newAuthSessionLifecycle(users authapplication.UserTokenVersionStore, tokenVersionCache authapplication.TokenVersionCache, sessions authapplication.RefreshSessionStore, passwordChangeSessions authapplication.PasswordChangeSessionStore, tokenVersions authvalidators.TokenVersionLocalInvalidator, settings serviceconfig.AuthSettings) (authsessions.Lifecycle, error) {
	return authsessions.NewLifecycle(users, tokenVersionCache, sessions, passwordChangeSessions, settings.MaxActiveSessionsPerUser, tokenVersions)
}

func newRefreshTokenSettings(settings serviceconfig.AuthSettings) authcommand.RefreshTokenSettings {
	return authcommand.RefreshTokenSettings{RefreshTokenRotation: settings.RefreshTokenRotation}
}

func newTokenVersionValidator(params TokenVersionValidatorParams) TokenVersionValidatorResult {
	validator := authvalidators.NewCachingValidator(params.Cache)
	return TokenVersionValidatorResult{Validator: authvalidators.NewMetricsTokenVersionValidator(validator, params.Metrics), Invalidator: validator}
}

// Provider：传输层

func newAuthController(params AuthControllerParams) *authhttp.AuthController {
	return authhttp.NewAuthController(authhttp.AuthControllerOptions{
		Login:          params.Login,
		Refresh:        params.Refresh,
		ChangePassword: params.ChangePassword,
		LogoutCurrent:  params.LogoutCurrent,
		LogoutAll:      params.LogoutAll,
		Validator:      params.Validator,
	})
}

// 运行时资源方法：token version 本地缓存

func (r *tokenVersionCacheResource) Close() error {
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

// 运行时资源方法：session purge workerpool

func (h *sessionPurgePoolHolder) Start(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pool != nil {
		return nil
	}
	pool, err := authredis.NewSessionPurgePool(h.log)
	if err != nil {
		return err
	}
	h.pool = pool
	return nil
}

func (h *sessionPurgePoolHolder) Stop(ctx context.Context) error {
	h.mu.Lock()
	pool := h.pool
	h.pool = nil
	h.mu.Unlock()
	if pool == nil {
		return nil
	}
	return pool.Stop(ctx)
}

func (h *sessionPurgePoolHolder) Submit(ctx context.Context, task workerpool.Task) error {
	h.mu.RLock()
	pool := h.pool
	h.mu.RUnlock()
	if pool == nil {
		return workerpool.ErrClosed
	}
	return pool.Submit(ctx, task)
}

func (h *sessionPurgePoolHolder) Stats() workerpool.Stats {
	h.mu.RLock()
	pool := h.pool
	h.mu.RUnlock()
	if pool == nil {
		return workerpool.Stats{Name: "auth.redis.session_purge", Closed: true}
	}
	return pool.Stats()
}

// 运行时资源方法：token version 本地缓存 holder

func (h *tokenVersionCacheHolder) Start(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.resource != nil {
		return nil
	}
	resource, err := newTokenVersionLocalCacheResource(h.cfg, h.users, h.cache)
	if err != nil {
		return err
	}
	h.resource = resource
	return nil
}

func (h *tokenVersionCacheHolder) GetOrLoad(ctx context.Context, userID string) (int64, error) {
	resource := h.currentResource()
	if resource == nil || resource.cache == nil {
		return 0, localcache.ErrClosed
	}
	return resource.cache.GetOrLoad(ctx, userID)
}

func (h *tokenVersionCacheHolder) Delete(userID string) error {
	resource := h.currentResource()
	if resource == nil || resource.cache == nil {
		return localcache.ErrClosed
	}
	return resource.cache.Delete(userID)
}

func (h *tokenVersionCacheHolder) Name() string {
	resource := h.currentResource()
	if resource == nil || resource.stats == nil {
		return authTokenVersionCacheName
	}
	return resource.stats.Name()
}

func (h *tokenVersionCacheHolder) Stats() localcache.Stats {
	resource := h.currentResource()
	if resource == nil || resource.stats == nil {
		return localcache.Stats{}
	}
	return resource.stats.Stats()
}

func (h *tokenVersionCacheHolder) Close(context.Context) error {
	h.mu.Lock()
	resource := h.resource
	h.resource = nil
	h.mu.Unlock()
	if resource == nil {
		return nil
	}
	return resource.Close()
}

func (h *tokenVersionCacheHolder) currentResource() *tokenVersionCacheResource {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.resource
}
