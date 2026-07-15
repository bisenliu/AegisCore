package permission

import (
	"go.uber.org/fx"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissioncommand "github.com/aegiscore/user-service/internal/features/permission/application/command"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
)

// Module 组装权限目录 feature 的应用服务、控制器和基础设施 adapter。
var Module = fx.Module("feature-permission",
	fx.Provide(
		// Fx 分类：横切能力 - permission feature 指标。
		newPermissionMetrics,
		// Fx 分类：Feature 基础设施 - Casbin policy 加载与用户角色解析。
		permissioncasbin.NewPolicyLoader,
		permissioncasbin.NewUserRoleResolver,
		// Fx 分类：横切能力 - Casbin reload 指标。
		commonmetrics.NewCasbinPolicyReloadMetrics,
		// Fx 分类：Feature 基础设施 - Casbin 引擎及其 port 投影。
		fx.Annotate(
			permissioncasbin.NewEngine,
			fx.As(fx.Self()),
			fx.As(new(permissionauthorization.Engine)),
			fx.As(new(permissionapplication.PolicyReloadEngine)),
		),
		// Fx 分类：横切能力 - 请求授权器。
		permissionauthorization.NewAuthorizer,
		// Fx 分类：Feature 基础设施 - 权限持久化与路由目录 adapter。
		fx.Annotate(permissionpostgres.NewPermissionStore, fx.As(new(permissionapplication.PermissionStore))),
		fx.Annotate(permissionhttp.NewRouteCatalogScanner, fx.As(new(permissionapplication.RouteCatalogScanner))),
		// Fx 分类：Feature 基础设施 - 分布式 policy version 同步 adapter。
		fx.Annotate(
			permissionredis.NewStore,
			fx.As(fx.Self()),
			fx.As(new(permissionapplication.PolicyVersionPublisher)),
		),
		fx.Annotate(
			permissionredis.NewVersionTracker,
			fx.As(fx.Self()),
			fx.As(new(permissionapplication.PolicyVersionTracker)),
		),
		// Fx 分类：Feature 应用 - policy 刷新编排与权限读写服务。
		fx.Annotate(permissionapplication.NewPolicyRefreshCoordinator, fx.As(new(permissionapplication.PolicyChangeNotifier))),
		permissioncommand.NewPermissionCommandService,
		permissionquery.NewPermissionQueryService,
		// Fx 分类：传输 - permission HTTP controller。
		permissionhttp.NewPermissionController,
		// Fx 分类：资源 - policy 变更后台 watcher。
		fx.Annotate(
			permissionredis.NewWatcher,
			fx.As(fx.Self()),
			fx.As(new(permissionredis.WatcherStatus)),
		),
	),
	fx.Invoke(
		// Fx 分类：生命周期 - 启动期完成 Casbin policy 初始加载。
		permissioncasbin.RegisterInitialLoad,
		// Fx 分类：生命周期 - 强制实例化 watcher 并注册启停 hook。
		func(*permissionredis.Watcher) {},
	),
)
