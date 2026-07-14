package auth

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/localcache"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
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

type loginUseCaseParams struct {
	fx.In

	Credentials authcredentials.Verifier
	Tokens      authtokens.Issuer
	Sessions    authsessions.Lifecycle
	Metrics     authapplication.Metrics `optional:"true"`
}

type refreshTokenUseCaseParams struct {
	fx.In

	Tokens   authtokens.Issuer
	Sessions authsessions.Lifecycle
	Config   *serviceconfig.Config
	Metrics  authapplication.Metrics `optional:"true"`
}

type changePasswordUseCaseParams struct {
	fx.In

	Credentials authcredentials.Verifier
	Tokens      authtokens.Issuer
	Sessions    authsessions.Lifecycle
	Metrics     authapplication.Metrics `optional:"true"`
}

type logoutCurrentSessionUseCaseParams struct {
	fx.In

	Sessions authsessions.Lifecycle
	Metrics  authapplication.Metrics `optional:"true"`
}

type logoutAllSessionsUseCaseParams struct {
	fx.In

	Sessions authsessions.Lifecycle
	Metrics  authapplication.Metrics `optional:"true"`
}

// Module 组装认证功能的应用服务、HTTP 传输层和基础设施适配器。
var Module = fx.Module("feature-auth",
	fx.Provide(
		// Fx 分类：横切能力 - auth feature 指标。
		newAuthMetrics,
		// Fx 分类：Feature 基础设施 - PostgreSQL 与 Redis port adapter。
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapplication.UserTokenVersionStore)),
			fx.As(new(authapplication.UserCredentialStore)),
		),
		fx.Annotate(
			authredis.NewSessionStore,
			fx.As(new(authapplication.TokenVersionCache)),
			fx.As(new(authapplication.RefreshSessionStore)),
			fx.As(new(authapplication.PasswordChangeSessionStore)),
		),
		// Fx 分类：资源 - auth feature 私有清理 worker pool。
		fx.Annotate(
			authredis.NewSessionPurgePool,
			fx.As(new(authredis.PurgeTaskPool)),
			fx.ResultTags(`name:"auth_session_purge_pool"`),
		),
		// Fx 分类：资源 - token version 本地缓存及其生命周期。
		newTokenVersionLocalCache,
		// Fx 分类：Feature 应用 - token、凭据和会话安全能力。
		newTokenVersionValidator,
		newCredentialVerifier,
		authtokens.NewIssuer,
		authtokens.NewAccessTokenVerifier,
		newAuthSessionLifecycle,
		// Fx 分类：Feature 应用 - 认证命令用例。
		newLoginUseCase,
		newRefreshTokenUseCase,
		newChangePasswordUseCase,
		newLogoutCurrentSessionUseCase,
		newLogoutAllSessionsUseCase,
		// Fx 分类：传输 - auth HTTP controller。
		authhttp.NewAuthController,
	),
)

func newCredentialVerifier(store authapplication.UserCredentialStore, passwordService *password.Service) authcredentials.Verifier {
	return authcredentials.NewVerifier(store, passwordService)
}

func newAuthSessionLifecycle(users authapplication.UserTokenVersionStore, tokenVersionCache authapplication.TokenVersionCache, sessions authapplication.RefreshSessionStore, passwordChangeSessions authapplication.PasswordChangeSessionStore, tokenVersions authvalidators.TokenVersionLocalInvalidator, cfg *serviceconfig.Config) authsessions.Lifecycle {
	return authsessions.NewLifecycle(users, tokenVersionCache, sessions, passwordChangeSessions, cfg.Auth.MaxActiveSessionsPerUser, tokenVersions)
}

func newLoginUseCase(params loginUseCaseParams) authcommand.LoginUseCase {
	return authcommand.NewLoginUseCase(authcommand.LoginDeps{
		Credentials: params.Credentials,
		Tokens:      params.Tokens,
		Sessions:    params.Sessions,
		Metrics:     params.Metrics,
	})
}

func newRefreshTokenUseCase(params refreshTokenUseCaseParams) authcommand.RefreshTokenUseCase {
	return authcommand.NewRefreshTokenUseCase(authcommand.RefreshTokenDeps{
		Tokens:   params.Tokens,
		Sessions: params.Sessions,
		Metrics:  params.Metrics,
		Settings: authcommand.RefreshTokenSettings{RefreshTokenRotation: params.Config.Auth.RefreshTokenRotation},
	})
}

func newChangePasswordUseCase(params changePasswordUseCaseParams) authcommand.ChangePasswordUseCase {
	return authcommand.NewChangePasswordUseCase(authcommand.ChangePasswordDeps{
		Credentials: params.Credentials,
		Tokens:      params.Tokens,
		Sessions:    params.Sessions,
		Metrics:     params.Metrics,
	})
}

func newLogoutCurrentSessionUseCase(params logoutCurrentSessionUseCaseParams) authcommand.LogoutCurrentSessionUseCase {
	return authcommand.NewLogoutCurrentSessionUseCase(authcommand.LogoutCurrentSessionDeps{
		Sessions: params.Sessions,
		Metrics:  params.Metrics,
	})
}

func newLogoutAllSessionsUseCase(params logoutAllSessionsUseCaseParams) authcommand.LogoutAllSessionsUseCase {
	return authcommand.NewLogoutAllSessionsUseCase(authcommand.LogoutAllSessionsDeps{
		Sessions: params.Sessions,
		Metrics:  params.Metrics,
	})
}

func newTokenVersionLocalCache(params tokenVersionCacheParams) (tokenVersionCacheResult, error) {
	cfg := params.Config.Auth.TokenVersionCache
	if !cfg.IsEnabled() {
		cache := authvalidators.NewDirectTokenVersionCache(params.Users, params.Cache)
		return tokenVersionCacheResult{Cache: cache, Stats: cache}, nil
	}
	cache, err := localcache.New[string, int64](localcache.Config[string]{
		Name:        authTokenVersionCacheName,
		Capacity:    cfg.SizeValue(),
		TTL:         cfg.TTLValue(),
		LoadTimeout: cfg.LoadTimeoutValue(),
		KeyString:   func(key string) string { return key },
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
