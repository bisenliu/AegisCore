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
	commonvalidation "github.com/aegiscore/common/validation"
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
	"github.com/aegiscore/user-service/internal/router"
)

// WiringModule 组装权限目录、请求授权和分布式 policy 同步的 provider，不主动注册 lifecycle。
var WiringModule = fx.Module(
	"feature-permission-wiring",
	permissionInternalModule,
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

var permissionInternalModule = fx.Module(
	"feature-permission-internal",
	permissionMetricsOptions,
	permissionStorageOptions,
	permissionAuthorizationOptions,
	permissionPolicySyncOptions,
	permissionApplicationOptions,
	permissionPublicOptions,
)

var permissionMetricsOptions = fx.Options(
	fx.Provide(
		newPermissionMetrics,
		commonmetrics.NewCasbinPolicyReloadMetrics,
		fx.Private,
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
		provideEngine,
		provideAuthorizer,
		fx.Private,
	),
)

var permissionPolicySyncOptions = fx.Options(
	fx.Provide(
		provideRedisStore,
		provideVersionTracker,
		providePolicyChangeNotifier,
		provideWatcher,
		fx.Private,
	),
)

var permissionApplicationOptions = fx.Options(
	fx.Provide(
		permissioncommand.NewPermissionCommandService,
		permissionquery.NewPermissionQueryService,
	),
)

var permissionPublicOptions = fx.Options(
	fx.Provide(
		providePermissionAuthorizer,
		providePermissionPolicyHealth,
		providePermissionPolicyWatcherStatus,
		providePermissionUserRoleCacheStats,
		providePermissionPolicyChangeNotifier,
		providePermissionPolicyInitializer,
		providePermissionApplicationWatcher,
		providePermissionUserRoleCacheCloser,
		providePermissionController,
	),
	fx.Provide(
		fx.Annotate(
			newPermissionRouteRegistrar,
			fx.As(new(router.AuthorizedRouteRegistrar)),
			fx.ResultTags(`group:"authorized_routes"`),
		),
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
	Stats    localcache.StatsSource               `name:"permission_rbac_user_roles_cache"`
	Closer   permissioncasbin.UserRoleCacheCloser `name:"permission_user_role_cache_closer"`
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
	Engine  permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Log     *zap.Logger
	Metrics permissionapplication.Metrics
}

type AuthorizerParams struct {
	fx.In

	Engine  permissionauthorization.Engine `name:"permission_authorization_engine"`
	Metrics permissionapplication.Metrics
}

type AuthorizerResult struct {
	fx.Out

	Authorizer permissionauthorization.Authorizer `name:"permission_authorizer"`
}

type PolicyChangeNotifierParams struct {
	fx.In

	Engine    permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Publisher permissionapplication.PolicyVersionPublisher
	Tracker   permissionapplication.PolicyVersionTracker
	Log       *zap.Logger
	Metrics   permissionapplication.Metrics
}

type PolicyChangeNotifierResult struct {
	fx.Out

	Notifier permissionapplication.PolicyChangeNotifier `name:"permission_policy_change_notifier"`
}

type PermissionAuthorizerParams struct {
	fx.In

	Authorizer permissionauthorization.Authorizer `name:"permission_authorizer"`
}

type PermissionPolicyHealthParams struct {
	fx.In

	Health permissionauthorization.PolicyHealth `name:"permission_policy_health"`
}

type PermissionPolicyWatcherStatusParams struct {
	fx.In

	Watcher permissionapplication.PolicyWatcherStatus `name:"permission_policy_watcher_status"`
}

type PermissionUserRoleCacheStatsParams struct {
	fx.In

	Stats localcache.StatsSource `name:"permission_rbac_user_roles_cache"`
}

type PermissionPolicyChangeNotifierParams struct {
	fx.In

	Notifier permissionapplication.PolicyChangeNotifier `name:"permission_policy_change_notifier"`
}

type PermissionUserRoleCacheCloserParams struct {
	fx.In

	Closer permissioncasbin.UserRoleCacheCloser `name:"permission_user_role_cache_closer"`
}

type PermissionUserRoleCacheStatsResult struct {
	fx.Out

	Stats localcache.StatsSource `name:"rbac_user_roles_cache"`
}

type PolicyEngineResult struct {
	fx.Out

	AuthorizationEngine permissionauthorization.Engine           `name:"permission_authorization_engine"`
	ReloadEngine        permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Health              permissionauthorization.PolicyHealth     `name:"permission_policy_health"`
	Initializer         permissionPolicyInitializer              `name:"permission_policy_initializer"`
}

type PolicyRedisStoreResult struct {
	fx.Out

	Store     *permissionredis.Store
	Publisher permissionapplication.PolicyVersionPublisher
}

type PolicyVersionTrackerResult struct {
	fx.Out

	Tracker *permissionredis.VersionTracker
	Port    permissionapplication.PolicyVersionTracker
}

type PolicyWatcherResult struct {
	fx.Out

	Watcher *permissionredis.Watcher
	Runner  permissionApplicationWatcher              `name:"permission_policy_watcher_runner"`
	Status  permissionapplication.PolicyWatcherStatus `name:"permission_policy_watcher_status"`
}

type RegisterRBACLifecycleParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Engine    permissionPolicyInitializer
	Watcher   permissionApplicationWatcher
	Closer    permissioncasbin.UserRoleCacheCloser
	Starter   userRoleResolverStarter `optional:"true"`
}

type permissionPolicyInitializer interface {
	InitializeFailClosed(context.Context)
}

type permissionApplicationWatcher interface {
	Start()
	Stop(context.Context) error
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

func provideEngine(loader permissioncasbin.Loader, metrics commonmetrics.ReloadMetrics, userRoles permissioncasbin.UserRoleResolver) PolicyEngineResult {
	engine := permissioncasbin.NewEngine(loader, metrics, userRoles)
	return PolicyEngineResult{AuthorizationEngine: engine, ReloadEngine: engine, Health: engine, Initializer: engine}
}

func provideRouteCatalogScanner(engine *gin.Engine) permissionapplication.RouteCatalogScanner {
	return permissionhttp.NewRouteCatalogScanner(engine)
}

func provideRedisStore(params CacheRedisParams, cfg *commonconfig.Config, log *zap.Logger) (PolicyRedisStoreResult, error) {
	store, err := permissionredis.NewStore(params.Client, cfg, log)
	if err != nil {
		return PolicyRedisStoreResult{}, err
	}
	return PolicyRedisStoreResult{Store: store, Publisher: store}, nil
}

func provideVersionTracker() PolicyVersionTrackerResult {
	tracker := permissionredis.NewVersionTracker()
	return PolicyVersionTrackerResult{Tracker: tracker, Port: tracker}
}

func provideAuthorizer(params AuthorizerParams) AuthorizerResult {
	return AuthorizerResult{Authorizer: permissionauthorization.NewAuthorizer(params.Engine, params.Metrics)}
}

func providePolicyChangeNotifier(params PolicyChangeNotifierParams) PolicyChangeNotifierResult {
	return PolicyChangeNotifierResult{Notifier: permissionapplication.NewPolicyRefreshCoordinator(params.Engine, params.Publisher, params.Tracker, params.Log, params.Metrics)}
}

func provideWatcher(params WatcherParams) PolicyWatcherResult {
	watcher := permissionredis.NewWatcher(permissionredis.WatcherParams{Store: params.Store, Tracker: params.Tracker, Engine: params.Engine, Log: params.Log, Metrics: params.Metrics})
	return PolicyWatcherResult{Watcher: watcher, Runner: watcher, Status: watcher}
}

func providePermissionAuthorizer(params PermissionAuthorizerParams) permissionauthorization.Authorizer {
	return params.Authorizer
}

func providePermissionPolicyHealth(params PermissionPolicyHealthParams) permissionauthorization.PolicyHealth {
	return params.Health
}

func providePermissionPolicyWatcherStatus(params PermissionPolicyWatcherStatusParams) permissionapplication.PolicyWatcherStatus {
	return params.Watcher
}

func providePermissionUserRoleCacheStats(params PermissionUserRoleCacheStatsParams) PermissionUserRoleCacheStatsResult {
	return PermissionUserRoleCacheStatsResult{Stats: params.Stats}
}

func providePermissionPolicyChangeNotifier(params PermissionPolicyChangeNotifierParams) permissionapplication.PolicyChangeNotifier {
	return params.Notifier
}

type PermissionPolicyInitializerParams struct {
	fx.In

	Initializer permissionPolicyInitializer `name:"permission_policy_initializer"`
}

func providePermissionPolicyInitializer(params PermissionPolicyInitializerParams) permissionPolicyInitializer {
	return params.Initializer
}

type PermissionApplicationWatcherParams struct {
	fx.In

	Watcher permissionApplicationWatcher `name:"permission_policy_watcher_runner"`
}

func providePermissionApplicationWatcher(params PermissionApplicationWatcherParams) permissionApplicationWatcher {
	return params.Watcher
}

func providePermissionUserRoleCacheCloser(params PermissionUserRoleCacheCloserParams) permissioncasbin.UserRoleCacheCloser {
	return params.Closer
}

func providePermissionController(command permissioncommand.PermissionCommandService, query permissionquery.PermissionQueryService, validator *commonvalidation.Validator) *permissionhttp.PermissionController {
	return permissionhttp.NewPermissionController(command, query, validator)
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
			params.Engine.InitializeFailClosed(ctx)
			params.Watcher.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return stopRBACLifecycle(ctx, params.Watcher.Stop, params.Closer)
		},
	})
}

func stopRBACLifecycle(ctx context.Context, stopWatcher func(context.Context) error, closer permissioncasbin.UserRoleCacheCloser) error {
	return errors.Join(
		stopWatcher(ctx),
		closer.Close(),
	)
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
