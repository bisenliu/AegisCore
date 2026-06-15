package permission

import (
	"go.uber.org/fx"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissioncommand "github.com/aegiscore/user-service/internal/features/permission/application/command"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
)

// Module 组装权限目录 feature 的应用服务、控制器和基础设施 adapter。
var Module = fx.Module("feature-permission",
	fx.Provide(
		permissioncasbin.NewPolicyLoader,
		permissioncasbin.NewEngine,
		fx.Annotate(permissionpostgres.NewPermissionStore, fx.As(new(permissionapplication.PermissionStore))),
		fx.Annotate(permissionhttp.NewRouteCatalogScanner, fx.As(new(permissionapplication.RouteCatalogScanner))),
		permissioncommand.NewPermissionCommandService,
		permissionquery.NewPermissionQueryService,
		permissionhttp.NewPermissionController,
	),
)
