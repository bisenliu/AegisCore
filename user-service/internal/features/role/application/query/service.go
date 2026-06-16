package query

import (
	"context"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

// RoleQueryService 定义角色管理读侧用例。
type RoleQueryService interface {
	ListRoles(ctx context.Context, query ListRolesQuery) (*ListRolesResult, error)
	GetRole(ctx context.Context, query GetRoleQuery) (*RoleResult, error)
	ListUserRoles(ctx context.Context, query UserRolesQuery) (*RolesResult, error)
	ListRolePermissions(ctx context.Context, query RolePermissionsQuery) (*PermissionsResult, error)
}

type roleQueryService struct {
	rolePermissions roleapplication.RolePermissionStore
	roles           roleapplication.RoleStore
	userRoles       roleapplication.UserRoleStore
}

// NewRoleQueryService 根据角色相关端口构造角色读侧服务。
func NewRoleQueryService(roles roleapplication.RoleStore, userRoles roleapplication.UserRoleStore, rolePermissions roleapplication.RolePermissionStore) RoleQueryService {
	return &roleQueryService{roles: roles, userRoles: userRoles, rolePermissions: rolePermissions}
}
