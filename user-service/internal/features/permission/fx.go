package permission

import (
	"context"
	"errors"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// WiringModule 组装权限目录、请求授权和分布式 policy 同步的 provider，不主动注册 lifecycle。
var WiringModule = fx.Module(
	"feature-permission-wiring",
	permissionMetricsOptions,
	permissionStorageOptions,
	permissionAuthorizationOptions,
	permissionPolicySyncOptions,
	permissionApplicationOptions,
	permissionTransportOptions,
)

// LifecycleModule 注册权限 feature 的运行时 lifecycle hook。
var LifecycleModule = fx.Module(
	"feature-permission-lifecycle",
	permissionLifecycleOptions,
)

// Module 组装权限目录、请求授权和分布式 policy 同步能力。
var Module = fx.Module(
	"feature-permission",
	WiringModule,
	LifecycleModule,
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
	Starter  userRoleResolverStarter
}

type userRoleResolverStarter interface {
	Start(context.Context) error
}

type userRoleResolverHolder struct {
	mu     sync.RWMutex
	params permissioncasbin.UserRoleResolverParams
	result permissioncasbin.UserRoleResolverResult
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
	Starter   userRoleResolverStarter `optional:"true"`
}

func providePermissionStore(params PrimaryDBParams) permissionapplication.PermissionStore {
	return permissionpostgres.NewPermissionStore(params.Client)
}

func providePolicyLoader(params PrimaryDBParams) permissioncasbin.Loader {
	return permissioncasbin.NewPolicyLoader(params.Client)
}

func provideUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	holder := &userRoleResolverHolder{params: permissioncasbin.UserRoleResolverParams{Config: params.Config, Client: params.Client}}
	return UserRoleResolverResult{Resolver: holder, Stats: holder, Closer: holder, Starter: holder}, nil
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
			if params.Starter != nil {
				if err := params.Starter.Start(ctx); err != nil {
					return err
				}
			} else if params.Closer == nil {
				return errors.New("rbac user role cache lifecycle dependency is required")
			}
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

func (h *userRoleResolverHolder) Start(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.result.Resolver != nil {
		return nil
	}
	result, err := permissioncasbin.NewUserRoleResolver(h.params)
	if err != nil {
		return err
	}
	h.result = result
	return nil
}

func (h *userRoleResolverHolder) RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	resolver := h.currentResolver()
	if resolver == nil {
		return nil, errors.New("rbac user role resolver is not started")
	}
	return resolver.RolesForUser(ctx, userID)
}

func (h *userRoleResolverHolder) InvalidateUserRole(userID uuid.UUID) {
	resolver := h.currentResolver()
	if resolver != nil {
		resolver.InvalidateUserRole(userID)
	}
}

func (h *userRoleResolverHolder) InvalidateAllUserRoles() {
	resolver := h.currentResolver()
	if resolver != nil {
		resolver.InvalidateAllUserRoles()
	}
}

func (h *userRoleResolverHolder) Close() error {
	h.mu.Lock()
	closer := h.result.Closer
	h.result = permissioncasbin.UserRoleResolverResult{}
	h.mu.Unlock()
	if closer == nil {
		return nil
	}
	return closer.Close()
}

func (h *userRoleResolverHolder) Name() string {
	h.mu.RLock()
	stats := h.result.Stats
	h.mu.RUnlock()
	if stats == nil {
		return "rbac_user_roles"
	}
	return stats.Name()
}

func (h *userRoleResolverHolder) Stats() localcache.Stats {
	h.mu.RLock()
	stats := h.result.Stats
	h.mu.RUnlock()
	if stats == nil {
		return localcache.Stats{}
	}
	return stats.Stats()
}

func (h *userRoleResolverHolder) currentResolver() permissioncasbin.UserRoleResolver {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.result.Resolver
}
