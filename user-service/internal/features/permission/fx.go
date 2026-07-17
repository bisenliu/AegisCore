package permission

import (
	"context"

	"github.com/gin-gonic/gin"
	rediscmd "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	commonconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	"github.com/aegiscore/user-service/ent"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissioncommand "github.com/aegiscore/user-service/internal/features/permission/application/command"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
)

// Module 组装权限目录、请求授权和分布式 policy 同步能力。
var Module = fx.Module(
	"feature-permission",
	permissionMetricsOptions,
	permissionStorageOptions,
	permissionAuthorizationOptions,
	permissionPolicySyncOptions,
	permissionApplicationOptions,
	permissionTransportOptions,
	permissionLifecycleOptions,
)

var permissionMetricsOptions = fx.Options(
	fx.Provide(
		newPermissionMetrics,
		commonmetrics.NewCasbinPolicyReloadMetrics,
	),
)

var permissionStorageOptions = fx.Options(
	fx.Provide(
		providePermissionStore,
		provideRouteCatalogScanner,
	),
)

var permissionAuthorizationOptions = fx.Options(
	fx.Provide(
		providePolicyLoader,
		provideUserRoleResolver,
		fx.Annotate(
			provideEngine,
			fx.As(fx.Self()),
			fx.As(new(permissionauthorization.Engine)),
			fx.As(new(permissionapplication.PolicyReloadEngine)),
		),
		permissionauthorization.NewAuthorizer,
	),
)

var permissionPolicySyncOptions = fx.Options(
	fx.Provide(
		fx.Annotate(
			provideRedisStore,
			fx.As(fx.Self()),
			fx.As(new(permissionapplication.PolicyVersionPublisher)),
		),
		fx.Annotate(
			permissionredis.NewVersionTracker,
			fx.As(fx.Self()),
			fx.As(new(permissionapplication.PolicyVersionTracker)),
		),
		providePolicyChangeNotifier,
		fx.Annotate(
			provideWatcher,
			fx.As(fx.Self()),
			fx.As(new(permissionredis.WatcherStatus)),
		),
	),
)

var permissionApplicationOptions = fx.Options(
	fx.Provide(
		permissioncommand.NewPermissionCommandService,
		permissionquery.NewPermissionQueryService,
	),
)

var permissionTransportOptions = fx.Options(
	fx.Provide(
		permissionhttp.NewPermissionController,
	),
)

var permissionLifecycleOptions = fx.Options(
	fx.Invoke(
		registerRBACLifecycle,
	),
)

type PrimaryDBParams struct {
	fx.In

	Client *ent.Client `name:"primary_db"`
}

type CacheRedisParams struct {
	fx.In

	Client *rediscmd.Client `name:"cache_redis"`
}

type UserRoleResolverParams struct {
	fx.In

	Config *serviceconfig.Config
	Client *ent.Client `name:"primary_db"`
}

type UserRoleResolverResult struct {
	fx.Out

	Resolver permissioncasbin.UserRoleResolver
	Stats    localcache.StatsSource `name:"rbac_user_roles_cache"`
	Closer   permissioncasbin.UserRoleCacheCloser
}

type WatcherParams struct {
	fx.In

	Store   *permissionredis.Store
	Tracker *permissionredis.VersionTracker
	Engine  permissionapplication.PolicyReloadEngine
	Log     *zap.Logger
	Metrics permissionapplication.Metrics
}

type RegisterRBACLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Engine    *permissioncasbin.Engine
	Watcher   *permissionredis.Watcher
	Closer    permissioncasbin.UserRoleCacheCloser
}

func providePermissionStore(params PrimaryDBParams) permissionapplication.PermissionStore {
	return permissionpostgres.NewPermissionStore(params.Client)
}

func providePolicyLoader(params PrimaryDBParams) permissioncasbin.Loader {
	return permissioncasbin.NewPolicyLoader(params.Client)
}

func provideUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	result, err := permissioncasbin.NewUserRoleResolver(permissioncasbin.UserRoleResolverParams{Config: params.Config, Client: params.Client})
	if err != nil {
		return UserRoleResolverResult{}, err
	}
	return UserRoleResolverResult{Resolver: result.Resolver, Stats: result.Stats, Closer: result.Closer}, nil
}

func provideEngine(loader permissioncasbin.Loader, metrics commonmetrics.ReloadMetrics, userRoles permissioncasbin.UserRoleResolver) *permissioncasbin.Engine {
	return permissioncasbin.NewEngine(loader, metrics, userRoles)
}

func provideRouteCatalogScanner(engine *gin.Engine) permissionapplication.RouteCatalogScanner {
	return permissionhttp.NewRouteCatalogScanner(engine)
}

func provideRedisStore(params CacheRedisParams, cfg *commonconfig.Config, log *zap.Logger) (*permissionredis.Store, error) {
	return permissionredis.NewStore(params.Client, cfg, log)
}

func providePolicyChangeNotifier(engine permissionapplication.PolicyReloadEngine, publisher permissionapplication.PolicyVersionPublisher, tracker permissionapplication.PolicyVersionTracker, log *zap.Logger, metrics permissionapplication.Metrics) permissionapplication.PolicyChangeNotifier {
	return permissionapplication.NewPolicyRefreshCoordinator(engine, publisher, tracker, log, metrics)
}

func provideWatcher(params WatcherParams) *permissionredis.Watcher {
	return permissionredis.NewWatcher(permissionredis.WatcherParams{Store: params.Store, Tracker: params.Tracker, Engine: params.Engine, Log: params.Log, Metrics: params.Metrics})
}

func registerRBACLifecycle(params RegisterRBACLifecycleParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := params.Engine.Initialize(ctx); err != nil {
				return err
			}
			params.Watcher.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			stopErr := params.Watcher.Stop(ctx)
			closeErr := params.Closer.Close()
			if stopErr != nil {
				return stopErr
			}
			return closeErr
		},
	})
}
