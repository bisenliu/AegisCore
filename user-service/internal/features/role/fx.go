package role

import (
	"context"

	"go.uber.org/fx"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
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
		fx.Annotate(newPolicyChangeNotifier, fx.As(new(rolecommand.PolicyChangeNotifier))),
		// Fx 分类：Feature 应用 - 角色命令与查询服务。
		rolecommand.NewRoleCommandService,
		rolequery.NewRoleQueryService,
		// Fx 分类：传输 - role HTTP controller。
		rolehttp.NewRoleController,
	),
)

type policyChangeNotifier struct {
	notifier permissionapplication.PolicyChangeNotifier
}

func newPolicyChangeNotifier(notifier permissionapplication.PolicyChangeNotifier) *policyChangeNotifier {
	return &policyChangeNotifier{notifier: notifier}
}

// NotifyPolicyChanged 将 role application 的通知端口转发到 permission feature 的 revision-aware 协调器。
func (n *policyChangeNotifier) NotifyPolicyChanged(ctx context.Context, revision int64, change permissionapplication.PolicyChange) error {
	return n.notifier.NotifyPolicyChanged(ctx, revision, change)
}

// NotifyUserRoleChanged 将 role application 的用户角色缓存失效端口转发到 permission feature 协调器。
func (n *policyChangeNotifier) NotifyUserRoleChanged(ctx context.Context, revision int64, change permissionapplication.PolicyChange) error {
	return n.notifier.NotifyUserRoleChanged(ctx, revision, change)
}
