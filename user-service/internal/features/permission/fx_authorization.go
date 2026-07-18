package permission

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/fx"

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
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	"github.com/aegiscore/user-service/internal/router"
)

// permissionAuthorizationOptions 组装内存授权引擎及其本地用户角色解析依赖。
var permissionAuthorizationOptions = fx.Options(
	fx.Provide(
		providePolicyLoader,
		provideUserRoleResolver,
		provideEngine,
		provideAuthorizer,
		fx.Private,
	),
)

// permissionPublicOptions 将 permission 内部命名组件投影为跨 feature 或 router 可消费的公开依赖。
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

type UserRoleResolverParams struct {
	fx.In

	Config *serviceconfig.Config
	Client *ent.Client `name:"primary_db"`
}

// UserRoleResolverResult 同时暴露 resolver、cache stats、closer 和启动器，确保 lazy 初始化仍由 lifecycle 控制。
type UserRoleResolverResult struct {
	fx.Out

	Resolver permissioncasbin.UserRoleResolver
	Stats    localcache.StatsSource               `name:"permission_rbac_user_roles_cache"`
	Closer   permissioncasbin.UserRoleCacheCloser `name:"permission_user_role_cache_closer"`
	Starter  userRoleResolverStarter
}

// userRoleResolverStarter 是 lifecycle 启动 holder 的最小接口，避免把 Fx lifecycle 传入 Casbin adapter。
type userRoleResolverStarter interface {
	Start(context.Context) error
}

// userRoleResolverHolder 延迟创建真实 resolver，使 Fx graph 构建阶段不会提前访问数据库或缓存资源。
type userRoleResolverHolder struct {
	mu     sync.RWMutex
	params permissioncasbin.UserRoleResolverParams
	result permissioncasbin.UserRoleResolverResult
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

// permissionPolicyInitializer 只暴露 fail-closed 初始化能力给 lifecycle hook。
type permissionPolicyInitializer interface {
	InitializeFailClosed(context.Context)
}

func providePolicyLoader(params PrimaryDBParams) permissioncasbin.Loader {
	return permissioncasbin.NewPolicyLoader(params.Client)
}

func provideUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	holder := &userRoleResolverHolder{params: permissioncasbin.UserRoleResolverParams{Config: params.Config, Client: params.Client}}
	return UserRoleResolverResult{Resolver: holder, Stats: holder, Closer: holder, Starter: holder}, nil
}

// provideEngine 将同一个 Casbin engine 按不同端口投影，保持授权、reload、健康检查和初始化使用同一份内存策略。
func provideEngine(loader permissioncasbin.Loader, metrics commonmetrics.ReloadMetrics, userRoles permissioncasbin.UserRoleResolver) PolicyEngineResult {
	engine := permissioncasbin.NewEngine(loader, metrics, userRoles)
	return PolicyEngineResult{AuthorizationEngine: engine, ReloadEngine: engine, Health: engine, Initializer: engine}
}

func provideAuthorizer(params AuthorizerParams) AuthorizerResult {
	return AuthorizerResult{Authorizer: permissionauthorization.NewAuthorizer(params.Engine, params.Metrics)}
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

// Start 在应用启动阶段创建真实 resolver，避免 graph 生成或 dry-run 时触发运行时资源访问。
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
