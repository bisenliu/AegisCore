package permission

import (
	"context"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/localcache"
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
	),
)

// permissionPublicOptions 将 permission 内部命名组件投影为跨 feature 或 router 可消费的公开依赖。
var permissionPublicOptions = fx.Options(
	fx.Provide(
		providePermissionUserRoleCacheStats,
		providePermissionController,
	),
)

// Fx 参数与结果：授权核心

// UserRoleResolverParams 汇集用户角色 resolver 的缓存配置与主库依赖。
type UserRoleResolverParams struct {
	fx.In

	Settings serviceconfig.RBACSettings
	Client   *ent.Client `name:"primary_db"`
}

// UserRoleResolverResult 同时暴露 resolver 和 cache stats。
type UserRoleResolverResult struct {
	fx.Out

	Resolver permissioncasbin.UserRoleResolver
	Stats    localcache.StatsSource `name:"permission_rbac_user_roles_cache"`
}

// AuthorizerParams 汇集授权服务使用的命名策略引擎和观测依赖。
type AuthorizerParams struct {
	fx.In

	Engine  permissionauthorization.Engine `name:"permission_authorization_engine"`
	Metrics permissionapplication.Metrics
}

// AuthorizerResult 以 feature 私有名称导出授权服务，防止 Fx 按接口类型误绑定其他实现。
type AuthorizerResult struct {
	fx.Out

	InternalAuthorizer permissionauthorization.Authorizer `name:"permission_authorizer"`
	Authorizer         permissionauthorization.Authorizer
}

// Fx 参数与结果：公开投影

// PermissionUserRoleCacheStatsParams 接收 permission feature 内部命名的缓存统计源。
type PermissionUserRoleCacheStatsParams struct {
	fx.In

	Stats localcache.StatsSource `name:"permission_rbac_user_roles_cache"`
}

// PermissionUserRoleCacheStatsResult 将缓存统计源投影为服务级统一名称。
type PermissionUserRoleCacheStatsResult struct {
	fx.Out

	Stats localcache.StatsSource `name:"rbac_user_roles_cache"`
}

// PolicyEngineResult 将同一个内存引擎投影为授权、reload、健康检查和初始化端口。
type PolicyEngineResult struct {
	fx.Out

	AuthorizationEngine permissionauthorization.Engine           `name:"permission_authorization_engine"`
	ReloadEngine        permissionapplication.PolicyReloadEngine `name:"permission_policy_reload_engine"`
	InternalHealth      permissionauthorization.PolicyHealth     `name:"permission_policy_health"`
	Health              permissionauthorization.PolicyHealth
	Initializer         permissionPolicyInitializer     `name:"permission_policy_initializer"`
	EngineLifecycle     permissionPolicyEngineLifecycle `name:"permission_policy_engine_lifecycle"`
}

// permissionPolicyInitializer 只暴露 fail-closed 初始化能力给 lifecycle hook。
type permissionPolicyInitializer interface {
	InitializeFailClosed(context.Context)
}

// permissionPolicyEngineLifecycle 只暴露 Casbin engine lifecycle root 控制面给 lifecycle hook。
type permissionPolicyEngineLifecycle interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// Provider：授权核心

func providePolicyLoader(params PrimaryDBParams) permissioncasbin.Loader {
	return permissioncasbin.NewPolicyLoader(params.Client)
}

func provideUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	result, err := permissioncasbin.NewUserRoleResolver(permissioncasbin.UserRoleResolverParams{Settings: params.Settings, Client: params.Client})
	if err != nil {
		return UserRoleResolverResult{}, err
	}
	return UserRoleResolverResult{Resolver: result.Resolver, Stats: result.Stats}, nil
}

// provideEngine 将同一个 Casbin engine 按不同端口投影，保持授权、reload、健康检查和初始化使用同一份内存策略。
func provideEngine(loader permissioncasbin.Loader, metrics permissioncasbin.ReloadMetrics, userRoles permissioncasbin.UserRoleResolver) PolicyEngineResult {
	engine := permissioncasbin.NewEngine(loader, metrics, userRoles)
	return PolicyEngineResult{AuthorizationEngine: engine, ReloadEngine: engine, InternalHealth: engine, Health: engine, Initializer: engine, EngineLifecycle: engine}
}

func provideAuthorizer(params AuthorizerParams) AuthorizerResult {
	authorizer := permissionauthorization.NewAuthorizer(params.Engine, params.Metrics)
	return AuthorizerResult{InternalAuthorizer: authorizer, Authorizer: authorizer}
}

// Provider：公开投影

func providePermissionUserRoleCacheStats(params PermissionUserRoleCacheStatsParams) PermissionUserRoleCacheStatsResult {
	return PermissionUserRoleCacheStatsResult{Stats: params.Stats}
}

func providePermissionController(query permissionquery.PermissionQueryService, validator *commonvalidation.Validator) *permissionhttp.PermissionController {
	return permissionhttp.NewPermissionController(query, validator)
}
