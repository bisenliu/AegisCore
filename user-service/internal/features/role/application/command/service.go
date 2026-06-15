package command

import roleapplication "github.com/aegiscore/user-service/internal/features/role/application"

type roleCommandService struct {
	permissions     roleapplication.PermissionLookup
	rolePermissions roleapplication.RolePermissionStore
	roles           roleapplication.RoleStore
	userRoles       roleapplication.UserRoleStore
}

// NewRoleCommandService 根据角色相关端口构造角色写侧服务。
func NewRoleCommandService(roles roleapplication.RoleStore, userRoles roleapplication.UserRoleStore, rolePermissions roleapplication.RolePermissionStore, permissions roleapplication.PermissionLookup) RoleCommandService {
	return &roleCommandService{roles: roles, userRoles: userRoles, rolePermissions: rolePermissions, permissions: permissions}
}
