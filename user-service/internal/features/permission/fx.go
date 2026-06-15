package permission

import (
	"go.uber.org/fx"

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
		permissioncasbin.NewPolicyLoader,
		permissioncasbin.NewEngine,
		newAuthorizationEngine,
		newPolicyReloadEngine,
		fx.Annotate(permissionauthorization.NewAuthorizer, fx.As(new(permissionauthorization.Authorizer))),
		fx.Annotate(permissionpostgres.NewPermissionStore, fx.As(new(permissionapplication.PermissionStore))),
		fx.Annotate(permissionhttp.NewRouteCatalogScanner, fx.As(new(permissionapplication.RouteCatalogScanner))),
		permissionredis.NewStore,
		permissionredis.NewVersionTracker,
		newPolicyVersionPublisher,
		newPolicyVersionTracker,
		fx.Annotate(permissionapplication.NewPolicyRefreshCoordinator, fx.As(new(permissionapplication.PolicyChangeNotifier))),
		permissioncommand.NewPermissionCommandService,
		permissionquery.NewPermissionQueryService,
		permissionhttp.NewPermissionController,
	),
	fx.Invoke(permissionredis.NewWatcher),
)

func newAuthorizationEngine(engine *permissioncasbin.Engine) permissionauthorization.Engine {
	return engine
}

func newPolicyReloadEngine(engine *permissioncasbin.Engine) permissionapplication.PolicyReloadEngine {
	return engine
}

func newPolicyVersionPublisher(store *permissionredis.Store) permissionapplication.PolicyVersionPublisher {
	return store
}

func newPolicyVersionTracker(tracker *permissionredis.VersionTracker) permissionapplication.PolicyVersionTracker {
	return tracker
}
