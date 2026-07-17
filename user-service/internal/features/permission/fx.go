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

type primaryDBIn struct {
	fx.In

	Client *ent.Client `name:"primary_db"`
}

type cacheRedisIn struct {
	fx.In

	Client *rediscmd.Client `name:"cache_redis"`
}

type userRoleResolverIn struct {
	fx.In

	Config *serviceconfig.Config
	Client *ent.Client `name:"primary_db"`
}

type userRoleResolverOut struct {
	fx.Out

	Resolver permissioncasbin.UserRoleResolver
	Stats    localcache.StatsSource `name:"rbac_user_roles_cache"`
	Closer   permissioncasbin.UserRoleCacheCloser
}

type engineOut struct {
	fx.Out

	Engine              *permissioncasbin.Engine
	AuthorizationEngine permissionauthorization.Engine
	ReloadEngine        permissionapplication.PolicyReloadEngine
}

type redisStoreOut struct {
	fx.Out

	Store     *permissionredis.Store
	Publisher permissionapplication.PolicyVersionPublisher
}

type versionTrackerOut struct {
	fx.Out

	Tracker     *permissionredis.VersionTracker
	TrackerPort permissionapplication.PolicyVersionTracker
}

type watcherOut struct {
	fx.Out

	Watcher *permissionredis.Watcher
	Status  permissionredis.WatcherStatus
}

type watcherIn struct {
	fx.In

	Store   *permissionredis.Store
	Tracker *permissionredis.VersionTracker
	Engine  permissionapplication.PolicyReloadEngine
	Log     *zap.Logger
	Metrics permissionapplication.Metrics
}

type rbacLifecycleIn struct {
	fx.In

	Lifecycle fx.Lifecycle
	Engine    *permissioncasbin.Engine
	Watcher   *permissionredis.Watcher
	Closer    permissioncasbin.UserRoleCacheCloser
}

// Module 组装权限目录 feature 的应用服务、控制器和基础设施 adapter。
var Module = fx.Module("feature-permission",
	fx.Provide(
		// Fx 分类：横切能力 - permission feature 指标。
		newPermissionMetrics,
		// Fx 分类：Feature 基础设施 - Casbin policy 加载与用户角色解析。
		providePolicyLoader,
		provideUserRoleResolver,
		// Fx 分类：横切能力 - Casbin reload 指标。
		commonmetrics.NewCasbinPolicyReloadMetrics,
		// Fx 分类：Feature 基础设施 - Casbin 引擎及其 port 投影。
		provideEngine,
		// Fx 分类：横切能力 - 请求授权器。
		permissionauthorization.NewAuthorizer,
		// Fx 分类：Feature 基础设施 - 权限持久化与路由目录 adapter。
		providePermissionStore,
		provideRouteCatalogScanner,
		// Fx 分类：Feature 基础设施 - 分布式 policy version 同步 adapter。
		provideRedisStore,
		provideVersionTracker,
		// Fx 分类：Feature 应用 - policy 刷新编排与权限读写服务。
		providePolicyChangeNotifier,
		permissioncommand.NewPermissionCommandService,
		permissionquery.NewPermissionQueryService,
		// Fx 分类：传输 - permission HTTP controller。
		permissionhttp.NewPermissionController,
		// Fx 分类：资源 - policy 变更后台 watcher。
		provideWatcher,
	),
	fx.Invoke(
		// Fx 分类：生命周期 - 组合层显式编排 RBAC 资源启停。
		registerRBACLifecycle,
	),
)

func providePermissionStore(in primaryDBIn) permissionapplication.PermissionStore {
	return permissionpostgres.NewPermissionStore(in.Client)
}

func providePolicyLoader(in primaryDBIn) permissioncasbin.Loader {
	return permissioncasbin.NewPolicyLoader(in.Client)
}

func provideUserRoleResolver(in userRoleResolverIn) (userRoleResolverOut, error) {
	result, err := permissioncasbin.NewUserRoleResolver(permissioncasbin.UserRoleResolverParams{Config: in.Config, Client: in.Client})
	if err != nil {
		return userRoleResolverOut{}, err
	}
	return userRoleResolverOut{Resolver: result.Resolver, Stats: result.Stats, Closer: result.Closer}, nil
}

func provideEngine(loader permissioncasbin.Loader, metrics commonmetrics.ReloadMetrics, userRoles permissioncasbin.UserRoleResolver) engineOut {
	engine := permissioncasbin.NewEngine(loader, metrics, userRoles)
	return engineOut{Engine: engine, AuthorizationEngine: engine, ReloadEngine: engine}
}

func provideRouteCatalogScanner(engine *gin.Engine) permissionapplication.RouteCatalogScanner {
	return permissionhttp.NewRouteCatalogScanner(engine)
}

func provideRedisStore(in cacheRedisIn, cfg *commonconfig.Config, log *zap.Logger) (redisStoreOut, error) {
	store, err := permissionredis.NewStore(in.Client, cfg, log)
	if err != nil {
		return redisStoreOut{}, err
	}
	return redisStoreOut{Store: store, Publisher: store}, nil
}

func provideVersionTracker() versionTrackerOut {
	tracker := permissionredis.NewVersionTracker()
	return versionTrackerOut{Tracker: tracker, TrackerPort: tracker}
}

func providePolicyChangeNotifier(engine permissionapplication.PolicyReloadEngine, publisher permissionapplication.PolicyVersionPublisher, tracker permissionapplication.PolicyVersionTracker, log *zap.Logger, metrics permissionapplication.Metrics) permissionapplication.PolicyChangeNotifier {
	return permissionapplication.NewPolicyRefreshCoordinator(engine, publisher, tracker, log, metrics)
}

func provideWatcher(in watcherIn) watcherOut {
	watcher := permissionredis.NewWatcher(permissionredis.WatcherParams{Store: in.Store, Tracker: in.Tracker, Engine: in.Engine, Log: in.Log, Metrics: in.Metrics})
	return watcherOut{Watcher: watcher, Status: watcher}
}

func registerRBACLifecycle(in rbacLifecycleIn) {
	in.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			_ = in.Engine.Initialize(ctx)
			in.Watcher.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			stopErr := in.Watcher.Stop(ctx)
			closeErr := in.Closer.Close()
			if stopErr != nil {
				return stopErr
			}
			return closeErr
		},
	})
}
