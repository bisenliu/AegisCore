package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	commonconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	"github.com/aegiscore/common/runtime/workerpool"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	commonvalidation "github.com/aegiscore/common/validation"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
)

var _ authcredentials.PasswordService = (*password.Service)(nil)

func TestNewTokenVersionLocalCacheUsesFeatureConfig(t *testing.T) {
	lifecycle := fxtest.NewLifecycle(t)
	ctrl := gomock.NewController(t)
	result, err := newTokenVersionLocalCache(TokenVersionLocalCacheParams{
		Lifecycle: lifecycle,
		Settings: serviceconfig.AuthSettings{TokenVersionCache: serviceconfig.FeatureCacheConfig{
			Enabled: true, Size: 123, TTL: time.Minute, LoadTimeout: time.Second,
		}},
		Users: NewMockUserTokenVersionStore(ctrl),
		Cache: NewMockTokenVersionCache(ctrl),
	})
	require.NoError(t, err)
	lifecycle.RequireStart()
	require.EqualValues(t, 123, result.Stats.Stats().Capacity)
	lifecycle.RequireStop()
}

func TestDisabledTokenVersionLocalCacheReadsThroughAndPreservesValidation(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	cache := NewMockTokenVersionCache(ctrl)
	gomock.InOrder(
		cache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
		users.EXPECT().GetTokenVersion(gomock.Any(), userID).Return(int64(7), nil),
		cache.EXPECT().CacheTokenVersion(gomock.Any(), userID.String(), int64(7)).Return(nil),
		cache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
		users.EXPECT().GetTokenVersion(gomock.Any(), userID).Return(int64(7), nil),
		cache.EXPECT().CacheTokenVersion(gomock.Any(), userID.String(), int64(7)).Return(nil),
	)
	result, err := newTokenVersionLocalCache(TokenVersionLocalCacheParams{
		Settings: serviceconfig.AuthSettings{TokenVersionCache: serviceconfig.FeatureCacheConfig{
			Enabled: false, Size: -1, TTL: -time.Second, LoadTimeout: -time.Second,
		}},
		Users: users,
		Cache: cache,
	})
	require.NoError(t, err)

	validator := authvalidators.NewCachingValidator(result.Cache)
	require.NoError(t, validator.ValidateTokenVersion(context.Background(), userID.String(), 7))
	require.Error(t, validator.ValidateTokenVersion(context.Background(), userID.String(), 8))
	require.NoError(t, validator.InvalidateTokenVersion(userID.String()))
	require.Equal(t, authTokenVersionCacheName, result.Stats.Name())
	require.EqualValues(t, 2, result.Stats.Stats().LoadSuccess)
	require.Zero(t, result.Stats.Stats().Capacity)
}

func TestTokenVersionLocalCacheResourceCloseIsIdempotent(t *testing.T) {
	resource, err := newTokenVersionLocalCacheResource(serviceconfig.FeatureCacheConfig{
		Enabled: true, Size: 10, TTL: time.Minute, LoadTimeout: time.Second,
	}, NewMockUserTokenVersionStore(gomock.NewController(t)), NewMockTokenVersionCache(gomock.NewController(t)))
	require.NoError(t, err)
	require.NotNil(t, resource.cache)
	require.EqualValues(t, 10, resource.stats.Stats().Capacity)

	require.NoError(t, resource.Close())
	require.NoError(t, resource.Close())
	require.ErrorIs(t, resource.cache.Delete("018f0000-0000-7000-8000-000000000502"), localcache.ErrClosed)
}

func TestTokenVersionLocalCacheRejectsNegativeCapacity(t *testing.T) {
	_, err := newTokenVersionLocalCacheResource(serviceconfig.FeatureCacheConfig{
		Enabled: true, Size: -1, TTL: time.Minute, LoadTimeout: time.Second,
	}, NewMockUserTokenVersionStore(gomock.NewController(t)), NewMockTokenVersionCache(gomock.NewController(t)))
	require.ErrorIs(t, err, localcache.ErrCapacityRequired)
}

func TestDisabledTokenVersionLocalCacheResourceCloseIsNoop(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000503")
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	cache := NewMockTokenVersionCache(ctrl)
	resource, err := newTokenVersionLocalCacheResource(serviceconfig.FeatureCacheConfig{
		Enabled: false, Size: -1, TTL: -time.Second, LoadTimeout: -time.Second,
	}, users, cache)
	require.NoError(t, err)

	require.NoError(t, resource.Close())
	require.NoError(t, resource.Close())
	gomock.InOrder(
		cache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
		users.EXPECT().GetTokenVersion(gomock.Any(), userID).Return(int64(9), nil),
		cache.EXPECT().CacheTokenVersion(gomock.Any(), userID.String(), int64(9)).Return(nil),
	)
	version, err := resource.cache.GetOrLoad(context.Background(), userID.String())
	require.NoError(t, err)
	require.EqualValues(t, 9, version)
	require.Equal(t, authTokenVersionCacheName, resource.stats.Name())
}

func TestAuthModuleBuildsCommandGraphWithMetricsConfigurations(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			outputs := newAuthModuleTestApp(t, enabled, true)

			require.NotNil(t, outputs.login)
			require.NotNil(t, outputs.refresh)
			require.NotNil(t, outputs.changePassword)
			require.NotNil(t, outputs.logoutCurrent)
			require.NotNil(t, outputs.logoutAll)
			require.NotNil(t, outputs.verifier)
			require.NotNil(t, outputs.refreshSessions)
			require.NotNil(t, outputs.passwordChangeSessions)
			require.NotNil(t, outputs.validator)
			require.NotNil(t, outputs.invalidator)
			require.NotNil(t, outputs.controller)
			require.True(t, outputs.settings.RefreshTokenRotation)

			if enabled {
				require.IsType(t, &prometheusMetrics{}, outputs.metrics)
				return
			}

			require.IsType(t, authapplication.NopMetrics(), outputs.metrics)
			require.Nil(t, outputs.provider.Gatherer())
		})
	}
}

func TestAuthModuleRefreshTokenSettingsFollowsConfig(t *testing.T) {
	for _, rotation := range []bool{true, false} {
		t.Run(map[bool]string{true: "rotation_enabled", false: "rotation_disabled"}[rotation], func(t *testing.T) {
			outputs := newAuthModuleTestApp(t, false, rotation)
			require.Equal(t, rotation, outputs.settings.RefreshTokenRotation)
		})
	}
}

func TestAuthModuleMetricsEdgesAreRequired(t *testing.T) {
	var login authcommand.LoginUseCase
	options := append(newAuthModuleBaseOptions(t, true, nil), fx.Populate(&login))
	app := fx.New(options...)

	require.Error(t, app.Err())
	require.Contains(t, app.Err().Error(), "metrics.Provider")
}

func TestAuthModuleCommandConstructorsHaveMetricsEdges(t *testing.T) {
	outputs := newAuthModuleTestApp(t, true, true)
	graphText := string(outputs.graph)
	for _, constructor := range []string{"NewLoginUseCase", "NewRefreshTokenUseCase", "NewChangePasswordUseCase", "NewLogoutCurrentSessionUseCase", "NewLogoutAllSessionsUseCase"} {
		match := regexp.MustCompile(`(constructor_\d+) \[shape=plaintext label="` + constructor + `"\];`).FindStringSubmatch(graphText)
		require.Len(t, match, 2, graphText)
		require.Contains(t, graphText, match[1]+` -> "application.Metrics" [ltail=`)
	}
}

func TestAuthModuleStopsAuthResourcesBeforeRedis(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	var purgePool authredis.PurgeTaskPool
	stopOrder := make([]string, 0, 3)
	options := append(newAuthModuleBaseOptionsWithoutRedis(t, true, newAuthModuleMetricsProvider(t, false)),
		fx.Provide(fx.Annotate(func(lifecycle fx.Lifecycle) *rediscache.Client {
			lifecycle.Append(fx.StopHook(func() error {
				require.NotNil(t, purgePool)
				require.True(t, purgePool.Stats().Closed,
					"purge pool must be stopped before redis client closes")
				stopOrder = append(stopOrder, "redis")
				return redisClient.Close()
			}))
			return redisClient
		}, fx.ResultTags(`name:"cache_redis"`))),
		fx.Invoke(func(params struct {
			fx.In

			Pool  authredis.PurgeTaskPool               `name:"auth_session_purge_pool"`
			Cache authvalidators.LocalTokenVersionCache `name:"auth_token_version_cache"`
		}) {
			require.NotNil(t, params.Pool)
			require.NotNil(t, params.Cache)
			purgePool = params.Pool
		}),
	)
	app := fxtest.New(t, options...)
	app.RequireStart().RequireStop()
	require.Equal(t, []string{"redis"}, stopOrder,
		"redis close hook did not run")
	require.Error(t, redisClient.Ping(context.Background()).Err(),
		"redis client should be closed by provider hook")
}

func TestAuthModuleStopsAuthResourcesWhenLaterStartHookFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	startErr := errors.New("later start failed")
	var purgePool authredis.PurgeTaskPool
	options := append(newAuthModuleBaseOptionsWithoutRedis(t, true, newAuthModuleMetricsProvider(t, false)),
		fx.Supply(fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`))),
		fx.Invoke(func(params struct {
			fx.In

			Pool  authredis.PurgeTaskPool               `name:"auth_session_purge_pool"`
			Cache authvalidators.LocalTokenVersionCache `name:"auth_token_version_cache"`
		}) {
			require.NotNil(t, params.Pool)
			require.NotNil(t, params.Cache)
			purgePool = params.Pool
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{OnStart: func(context.Context) error { return startErr }})
		}),
	)
	app := fxtest.New(t, options...)
	startCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := app.Start(startCtx)
	require.ErrorIs(t, err, startErr)
	require.NotNil(t, purgePool)
	require.True(t, purgePool.Stats().Closed)
	require.NoError(t, redisClient.Ping(context.Background()).Err())
	require.NoError(t, redisClient.Close())
}

func TestSessionPurgePoolHolderRejectsBeforeStartAndStopsIdempotently(t *testing.T) {
	holder := &sessionPurgePoolHolder{log: zap.NewNop()}
	require.ErrorIs(t, holder.Submit(context.Background(), workerpool.Task{Name: "test", Run: func(context.Context) error { return nil }}), workerpool.ErrClosed)
	require.True(t, holder.Stats().Closed)
	require.NoError(t, holder.Start(context.Background()))
	require.False(t, holder.Stats().Closed)
	require.NoError(t, holder.Stop(context.Background()))
	require.NoError(t, holder.Stop(context.Background()))
	require.True(t, holder.Stats().Closed)
}

func TestTokenVersionLocalCacheHolderFailsClosedBeforeStartAndClosesIdempotently(t *testing.T) {
	holder := &tokenVersionLocalCacheHolder{cfg: serviceconfig.FeatureCacheConfig{Enabled: true, Size: 10, TTL: time.Minute, LoadTimeout: time.Second}, users: NewMockUserTokenVersionStore(gomock.NewController(t)), cache: NewMockTokenVersionCache(gomock.NewController(t))}
	require.ErrorIs(t, holder.Delete("018f0000-0000-7000-8000-000000000504"), localcache.ErrClosed)
	_, err := holder.GetOrLoad(context.Background(), "018f0000-0000-7000-8000-000000000504")
	require.ErrorIs(t, err, localcache.ErrClosed)
	require.NoError(t, holder.Start(context.Background()))
	require.EqualValues(t, 10, holder.Stats().Capacity)
	require.NoError(t, holder.Close(context.Background()))
	require.NoError(t, holder.Close(context.Background()))
	require.ErrorIs(t, holder.Delete("018f0000-0000-7000-8000-000000000504"), localcache.ErrClosed)
}

type authModuleOutputs struct {
	provider               *commonmetrics.Provider
	login                  authcommand.LoginUseCase
	refresh                authcommand.RefreshTokenUseCase
	changePassword         authcommand.ChangePasswordUseCase
	logoutCurrent          authcommand.LogoutCurrentSessionUseCase
	logoutAll              authcommand.LogoutAllSessionsUseCase
	verifier               authcredentials.Verifier
	settings               authcommand.RefreshTokenSettings
	metrics                authapplication.Metrics
	refreshSessions        authapplication.RefreshSessionStore
	passwordChangeSessions authapplication.PasswordChangeSessionStore
	validator              commonauth.TokenVersionValidator
	invalidator            authvalidators.TokenVersionLocalInvalidator
	controller             *authhttp.AuthController
	graph                  fx.DotGraph
}

func newAuthModuleTestApp(t *testing.T, metricsEnabled bool, refreshRotation bool) authModuleOutputs {
	t.Helper()
	provider := newAuthModuleMetricsProvider(t, metricsEnabled)
	outputs := authModuleOutputs{provider: provider}
	options := append(newAuthModuleBaseOptions(t, refreshRotation, provider),
		fx.Populate(
			&outputs.login,
			&outputs.refresh,
			&outputs.changePassword,
			&outputs.logoutCurrent,
			&outputs.logoutAll,
			&outputs.verifier,
			&outputs.settings,
			&outputs.metrics,
			&outputs.refreshSessions,
			&outputs.passwordChangeSessions,
			&outputs.validator,
			&outputs.invalidator,
			&outputs.controller,
			&outputs.graph,
		),
	)
	app := fxtest.New(t, options...)
	app.RequireStart().RequireStop()
	return outputs
}

func newAuthModuleBaseOptions(t *testing.T, refreshRotation bool, provider *commonmetrics.Provider) []fx.Option {
	t.Helper()
	redisServer := miniredis.RunT(t)
	redisClient := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	options := newAuthModuleBaseOptionsWithoutRedis(t, refreshRotation, provider)
	return append(options, fx.Supply(fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`))))
}

func newAuthModuleBaseOptionsWithoutRedis(t *testing.T, refreshRotation bool, provider *commonmetrics.Provider) []fx.Option {
	t.Helper()
	settings := serviceconfig.AuthSettings{
		AppName: "aegiscore-user-service-module-test",
		JWT: serviceconfig.JWTConfig{
			Secret:                 "auth-module-test-secret-32-bytes",
			Issuer:                 "issuer",
			Audience:               "audience",
			AccessTokenTTL:         15 * time.Minute,
			RefreshTokenTTL:        time.Hour,
			PasswordChangeTokenTTL: 5 * time.Minute,
		},
		TokenVersionCache:        serviceconfig.FeatureCacheConfig{Enabled: false},
		TokenVersionCacheTTL:     time.Minute,
		RefreshTokenRotation:     refreshRotation,
		MaxActiveSessionsPerUser: 5,
	}
	jwtService := commonauth.NewJWTService(commonauth.JWTConfig{Secret: settings.JWT.Secret, Issuer: settings.JWT.Issuer, Audience: settings.JWT.Audience})
	passwordService, err := password.NewService()
	require.NoError(t, err)
	validator, err := commonvalidation.NewDefault()
	require.NoError(t, err)
	credentialStore := authModuleCredentialStore{}

	options := []fx.Option{
		fxtest.WithTestLogger(t),
		fx.Supply(
			settings,
			jwtService,
			passwordService,
			validator,
			zap.NewNop(),
		),
		Module,
		fx.Replace(
			fx.Annotate(credentialStore, fx.As(new(authapplication.UserCredentialStore))),
			fx.Annotate(credentialStore, fx.As(new(authapplication.UserTokenVersionStore))),
		),
	}
	if provider != nil {
		options = append([]fx.Option{fx.Supply(provider)}, options...)
	}
	return options
}

func newAuthModuleMetricsProvider(t *testing.T, enabled bool) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      commonconfig.MetricsConfig{Enabled: enabled},
		ServiceName: "aegiscore-user-service-module-test",
		Environment: "test",
	})
	require.NoError(t, err)
	return provider
}

type authModuleCredentialStore struct{}

func (authModuleCredentialStore) GetByUsername(context.Context, string) (*authdomain.UserCredential, error) {
	return nil, authdomain.ErrInvalidCredentials
}

func (authModuleCredentialStore) GetCredentialByUserID(context.Context, uuid.UUID) (*authdomain.UserCredential, error) {
	return nil, authdomain.ErrInvalidCredentials
}

func (authModuleCredentialStore) UpdateCredentials(context.Context, authdomain.UpdateCredentialsInput) (int64, error) {
	return 0, authdomain.ErrInvalidCredentials
}

func (authModuleCredentialStore) GetTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 1, nil
}

func (authModuleCredentialStore) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 2, nil
}
