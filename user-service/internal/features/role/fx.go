package role

import (
	"go.uber.org/fx"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
)

// Module 组装角色管理 feature 的应用服务、控制器和基础设施 adapter。
var Module = fx.Module("feature-role",
	fx.Provide(
		fx.Annotate(rolepostgres.NewRoleStore, fx.As(new(roleapplication.RoleStore))),
		fx.Annotate(rolepostgres.NewUserRoleStore, fx.As(new(roleapplication.UserRoleStore))),
		fx.Annotate(rolepostgres.NewRolePermissionStore, fx.As(new(roleapplication.RolePermissionStore))),
		fx.Annotate(rolepostgres.NewPermissionLookup, fx.As(new(roleapplication.PermissionLookup))),
		rolecommand.NewRoleCommandService,
		rolequery.NewRoleQueryService,
		rolehttp.NewRoleController,
	),
)
