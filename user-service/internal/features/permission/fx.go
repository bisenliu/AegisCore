package permission

import (
	"go.uber.org/fx"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
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

// permissionMetricsOptions 只注册 permission feature 内部指标与 Casbin reload 指标。
var permissionMetricsOptions = fx.Options(
	fx.Provide(
		newPermissionMetrics,
		commonmetrics.NewCasbinPolicyReloadMetrics,
		fx.Private,
	),
)

// permissionApplicationOptions 注册权限目录只读 use case，具体端口实现由同包其他 Fx 文件提供。
var permissionApplicationOptions = fx.Options(
	fx.Provide(
		permissionquery.NewPermissionQueryService,
	),
)
