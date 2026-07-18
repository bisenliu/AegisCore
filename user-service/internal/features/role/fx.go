package role

import (
	"go.uber.org/fx"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	"github.com/aegiscore/user-service/internal/router"
)

// Module 组装角色管理 feature 的应用服务、控制器和基础设施 adapter。
var Module = fx.Module("feature-role",
	fx.Provide(
		// Fx 分类：Feature 基础设施 - PostgreSQL port adapter。
		fx.Annotate(
			rolepostgres.NewRoleStore,
			fx.ParamTags(`name:"primary_db"`),
			fx.As(new(roleapplication.RoleStore)),
		),
		fx.Annotate(
			rolepostgres.NewUserRoleStore,
			fx.ParamTags(`name:"primary_db"`),
			fx.As(new(roleapplication.UserRoleStore)),
		),
		fx.Annotate(
			rolepostgres.NewRolePermissionStore,
			fx.ParamTags(`name:"primary_db"`),
			fx.As(new(roleapplication.RolePermissionStore)),
		),
		fx.Annotate(rolepostgres.NewPermissionLookup, fx.As(new(roleapplication.PermissionLookup))),
		// Fx 分类：Feature 应用 - 角色命令与查询服务。
		rolecommand.NewRoleCommandService,
		rolequery.NewRoleQueryService,
		// Fx 分类：传输 - role HTTP controller。
		rolehttp.NewRoleController,
		fx.Annotate(
			newRoleRouteRegistrar,
			fx.As(new(router.AuthorizedRouteRegistrar)),
			fx.ResultTags(`group:"authorized_routes"`),
		),
	),
)
