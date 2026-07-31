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
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

// Fx 选项

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
		newPermissionRuntime,
		providePermissionAuthorizer,
		providePermissionPolicyHealth,
		providePermissionPolicyWatcherStatus,
		providePermissionOutboxDispatcherStatus,
		providePermissionUserRoleCacheStats,
		providePermissionPolicyChangeNotifier,
		providePermissionController,
	),
)

// Fx 参数与结果：授权核心

type UserRoleResolverParams struct {
	fx.In

	Settings serviceconfig.RBACSettings
	Client   *ent.Client `name:"primary_db"`
}

// UserRoleResolverResult 同时暴露 resolver、cache stats、closer 和生命周期视图，确保 lazy 初始化仍由 lifecycle 控制。
type UserRoleResolverResult struct {
	fx.Out

	Resolver  permissioncasbin.UserRoleResolver
	Stats     localcache.StatsSource               `name:"permission_rbac_user_roles_cache"`
	Closer    permissioncasbin.UserRoleCacheCloser `name:"permission_user_role_cache_closer"`
	Lifecycle userRoleResolverLifecycle            `name:"permission_user_role_resolver_lifecycle"`
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

// Fx 参数与结果：公开投影

type PermissionUserRoleCacheStatsParams struct {
	fx.In

	Stats localcache.StatsSource `name:"permission_rbac_user_roles_cache"`
}

type PermissionUserRoleCacheStatsResult struct {
	fx.Out

	Stats localcache.StatsSource `name:"rbac_user_roles_cache"`
}

// Fx 参数与结果：运行时聚合

type PermissionRuntimeParams struct {
	fx.In

	Authorizer     permissionauthorization.Authorizer           `name:"permission_authorizer"`
	Health         permissionauthorization.PolicyHealth         `name:"permission_policy_health"`
	Watcher        policyWatcherRunner                          `name:"permission_policy_watcher_runner"`
	Status         permissionapplication.PolicyWatcherStatus    `name:"permission_policy_watcher_status"`
	Dispatcher     permissionapplication.OutboxDispatcherRunner `name:"permission_outbox_dispatcher_runner"`
	DispatchStatus permissionapplication.OutboxDispatcherStatus `name:"permission_outbox_dispatcher_status"`
	Notifier       permissionapplication.PolicyChangeNotifier   `name:"permission_policy_change_notifier"`
	Initializer    permissionPolicyInitializer                  `name:"permission_policy_initializer"`
	UserRoles      userRoleResolverLifecycle                    `name:"permission_user_role_resolver_lifecycle"`
}

type PolicyEngineResult struct {
	fx.Out

	AuthorizationEngine permissionauthorization.Engine           `name:"permission_authorization_engine"`
	ReloadEngine        permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	Health              permissionauthorization.PolicyHealth     `name:"permission_policy_health"`
	Initializer         permissionPolicyInitializer              `name:"permission_policy_initializer"`
}

// 内部运行时类型

// userRoleResolverLifecycle 是 permission composition 内部显式启停契约，避免把启动能力隐藏在关闭接口中。
type userRoleResolverLifecycle interface {
	Start(context.Context) error
	Close() error
}

// userRoleResolverHolder 延迟创建真实 resolver，使 Fx graph 构建阶段不会提前访问数据库或缓存资源。
type userRoleResolverHolder struct {
	mu     sync.RWMutex
	params permissioncasbin.UserRoleResolverParams
	result permissioncasbin.UserRoleResolverResult
}

// PermissionRuntime 聚合 permission feature 对外稳定 RBAC runtime 组件，避免 public 投影散落在多个 named 转发函数中。
type PermissionRuntime struct {
	Authorizer       permissionauthorization.Authorizer
	PolicyHealth     permissionauthorization.PolicyHealth
	WatcherStatus    permissionapplication.PolicyWatcherStatus
	DispatcherStatus permissionapplication.OutboxDispatcherStatus
	Notifier         permissionapplication.PolicyChangeNotifier
	Initializer      permissionPolicyInitializer
	Watcher          policyWatcherRunner
	Dispatcher       permissionapplication.OutboxDispatcherRunner
	UserRoles        userRoleResolverLifecycle
}

// permissionPolicyInitializer 只暴露 fail-closed 初始化能力给 lifecycle hook。
type permissionPolicyInitializer interface {
	InitializeFailClosed(context.Context)
}

// Provider：授权核心

func providePolicyLoader(params PrimaryDBParams) permissioncasbin.Loader {
	return permissioncasbin.NewPolicyLoader(params.Client)
}

func provideUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	holder := &userRoleResolverHolder{params: permissioncasbin.UserRoleResolverParams{Settings: params.Settings, Client: params.Client}}
	return UserRoleResolverResult{Resolver: holder, Stats: holder, Closer: holder, Lifecycle: holder}, nil
}

// provideEngine 将同一个 Casbin engine 按不同端口投影，保持授权、reload、健康检查和初始化使用同一份内存策略。
func provideEngine(loader permissioncasbin.Loader, metrics commonmetrics.ReloadMetrics, userRoles permissioncasbin.UserRoleResolver) PolicyEngineResult {
	engine := permissioncasbin.NewEngine(loader, metrics, userRoles)
	return PolicyEngineResult{AuthorizationEngine: engine, ReloadEngine: engine, Health: engine, Initializer: engine}
}

func provideAuthorizer(params AuthorizerParams) AuthorizerResult {
	return AuthorizerResult{Authorizer: permissionauthorization.NewAuthorizer(params.Engine, params.Metrics)}
}

// Provider：公开投影

func newPermissionRuntime(params PermissionRuntimeParams) *PermissionRuntime {
	return &PermissionRuntime{
		Authorizer:       params.Authorizer,
		PolicyHealth:     params.Health,
		WatcherStatus:    params.Status,
		DispatcherStatus: params.DispatchStatus,
		Notifier:         params.Notifier,
		Initializer:      params.Initializer,
		Watcher:          params.Watcher,
		Dispatcher:       params.Dispatcher,
		UserRoles:        params.UserRoles,
	}
}

func providePermissionAuthorizer(runtime *PermissionRuntime) permissionauthorization.Authorizer {
	return runtime.Authorizer
}

func providePermissionPolicyHealth(runtime *PermissionRuntime) permissionauthorization.PolicyHealth {
	return runtime.PolicyHealth
}

func providePermissionPolicyWatcherStatus(runtime *PermissionRuntime) permissionapplication.PolicyWatcherStatus {
	return runtime.WatcherStatus
}

func providePermissionOutboxDispatcherStatus(runtime *PermissionRuntime) permissionapplication.OutboxDispatcherStatus {
	return runtime.DispatcherStatus
}

func providePermissionUserRoleCacheStats(params PermissionUserRoleCacheStatsParams) PermissionUserRoleCacheStatsResult {
	return PermissionUserRoleCacheStatsResult{Stats: params.Stats}
}

func providePermissionPolicyChangeNotifier(runtime *PermissionRuntime) permissionapplication.PolicyChangeNotifier {
	return runtime.Notifier
}

func providePermissionController(query permissionquery.PermissionQueryService, validator *commonvalidation.Validator) *permissionhttp.PermissionController {
	return permissionhttp.NewPermissionController(query, validator)
}

// 运行时资源方法：用户角色 resolver holder

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
