package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/workerpool"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	commonvalidation "github.com/aegiscore/common/validation"
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
	"github.com/aegiscore/user-service/internal/persistence/ent"
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
		authcommand.NewForceChangePasswordUseCase,
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

	Settings serviceconfig.AuthSettings
	Users    authapplication.UserTokenVersionStore
	Cache    authapplication.TokenVersionCache
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

	Login         authcommand.LoginUseCase
	Refresh       authcommand.RefreshTokenUseCase
	ForceChange   authcommand.ForceChangePasswordUseCase
	LogoutCurrent authcommand.LogoutCurrentSessionUseCase
	LogoutAll     authcommand.LogoutAllSessionsUseCase
	Validator     *commonvalidation.Validator
}

// 运行时资源 holder

type sessionPurgePoolHolder struct {
	mu   sync.RWMutex
	log  *zap.Logger
	pool *workerpool.Pool
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
	cfg := params.Settings.TokenVersionCache
	if !cfg.Enabled {
		direct := authvalidators.NewDirectTokenVersionCache(params.Users, params.Cache)
		return TokenVersionLocalCacheResult{Cache: direct, Stats: direct}, nil
	}
	// 每个副本独立维护该缓存；撤销只主动清理当前副本，其他副本在 TTL 到期后经 Redis 或数据库收敛。
	// cfg.TTL 表示正常路径可接受的 logout-all/改密跨副本撤销收敛窗口，调整时应重新评估撤销 SLA。
	// 是否使用事务 outbox 取决于 Redis token version 投影失败后是否需要可靠补偿，与固定 TTL 阈值无关。
	local, err := localcache.NewLoadingCache(cfg.Localcache(authTokenVersionCacheName), func(ctx context.Context, userID string) (int64, error) {
		parsedUserID, err := uuid.Parse(userID)
		if err != nil {
			return 0, fmt.Errorf("parse auth token version cache user id: %w", err)
		}
		return authvalidators.Current(ctx, params.Users, params.Cache, parsedUserID)
	})
	if err != nil {
		return TokenVersionLocalCacheResult{}, fmt.Errorf("create auth token version localcache: %w", err)
	}
	return TokenVersionLocalCacheResult{Cache: local, Stats: local}, nil
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
		Login:         params.Login,
		Refresh:       params.Refresh,
		ForceChange:   params.ForceChange,
		LogoutCurrent: params.LogoutCurrent,
		LogoutAll:     params.LogoutAll,
		Validator:     params.Validator,
	})
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
