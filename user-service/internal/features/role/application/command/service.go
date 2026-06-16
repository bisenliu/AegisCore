package command

import (
	"context"

	"go.uber.org/fx"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

type roleCommandService struct {
	policyChanges   permissionapplication.PolicyChangeNotifier
	permissions     roleapplication.PermissionLookup
	rolePermissions roleapplication.RolePermissionStore
	roles           roleapplication.RoleStore
	userRoles       roleapplication.UserRoleStore
}

// RoleCommandParams 包含角色写侧服务依赖。
type RoleCommandParams struct {
	fx.In

	Permissions     roleapplication.PermissionLookup
	PolicyChanges   permissionapplication.PolicyChangeNotifier
	RolePermissions roleapplication.RolePermissionStore
	Roles           roleapplication.RoleStore
	UserRoles       roleapplication.UserRoleStore
}

// NewRoleCommandService 根据角色相关端口构造角色写侧服务。
func NewRoleCommandService(params RoleCommandParams) RoleCommandService {
	return &roleCommandService{roles: params.Roles, userRoles: params.UserRoles, rolePermissions: params.RolePermissions, permissions: params.Permissions, policyChanges: params.PolicyChanges}
}

func (s *roleCommandService) notifyPolicyChanged(ctx context.Context, reason string) {
	if s.policyChanges == nil {
		return
	}
	s.policyChanges.NotifyPolicyChanged(ctx, reason)
}
