package command

import (
	"context"

	"github.com/google/uuid"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

// UserRoleCommand 包含单个用户角色绑定变更输入。
type UserRoleCommand struct {
	UserID uuid.UUID
	RoleID uuid.UUID
}

// ReplaceUserRolesCommand 包含替换用户角色绑定输入。
type ReplaceUserRolesCommand struct {
	UserID  uuid.UUID
	RoleIDs []uuid.UUID
}

// RolePermissionCommand 包含单个角色权限绑定变更输入。
type RolePermissionCommand struct {
	RoleID       uuid.UUID
	PermissionID uuid.UUID
}

// ReplaceRolePermissionsCommand 包含替换角色权限绑定输入。
type ReplaceRolePermissionsCommand struct {
	RoleID        uuid.UUID
	PermissionIDs []uuid.UUID
}

// AddUserRole 为用户新增角色绑定；重复绑定由 store 映射为明确冲突语义。
func (s *roleCommandService) AddUserRole(ctx context.Context, cmd UserRoleCommand) (*RolesResult, error) {
	if _, err := s.roles.GetByRoleID(ctx, cmd.RoleID); err != nil {
		return nil, err
	}
	if err := s.userRoles.Add(ctx, cmd.UserID, cmd.RoleID); err != nil {
		return nil, err
	}
	s.notifyPolicyChanged(ctx, "user_role_added")
	return s.listUserRoles(ctx, cmd.UserID)
}

// ReplaceUserRoles 幂等替换用户的完整角色绑定集合。
func (s *roleCommandService) ReplaceUserRoles(ctx context.Context, cmd ReplaceUserRolesCommand) (*RolesResult, error) {
	roleIDs := uniqueUUIDs(cmd.RoleIDs)
	if _, err := s.roles.GetByRoleIDs(ctx, roleIDs); err != nil {
		return nil, err
	}
	items, err := s.userRoles.Replace(ctx, cmd.UserID, roleIDs)
	if err != nil {
		return nil, err
	}
	s.notifyPolicyChanged(ctx, "user_roles_replaced")
	return &RolesResult{Items: items}, nil
}

// RemoveUserRole 删除用户角色绑定；不存在由 store 映射为明确 not found 语义。
func (s *roleCommandService) RemoveUserRole(ctx context.Context, cmd UserRoleCommand) (*RolesResult, error) {
	if err := s.userRoles.Remove(ctx, cmd.UserID, cmd.RoleID); err != nil {
		return nil, err
	}
	s.notifyPolicyChanged(ctx, "user_role_removed")
	return s.listUserRoles(ctx, cmd.UserID)
}

// AddRolePermission 为角色新增权限绑定；重复绑定由 store 映射为明确冲突语义。
func (s *roleCommandService) AddRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error) {
	if _, err := s.roles.GetByRoleID(ctx, cmd.RoleID); err != nil {
		return nil, err
	}
	permission, err := s.permissions.GetActiveByPermissionID(ctx, cmd.PermissionID)
	if err != nil {
		return nil, err
	}
	if err := s.rolePermissions.Add(ctx, cmd.RoleID, *permission); err != nil {
		return nil, err
	}
	s.notifyPolicyChanged(ctx, "role_permission_added")
	return s.listRolePermissions(ctx, cmd.RoleID)
}

// ReplaceRolePermissions 幂等替换角色的完整权限绑定集合。
func (s *roleCommandService) ReplaceRolePermissions(ctx context.Context, cmd ReplaceRolePermissionsCommand) (*PermissionsResult, error) {
	if _, err := s.roles.GetByRoleID(ctx, cmd.RoleID); err != nil {
		return nil, err
	}
	permissionIDs := uniqueUUIDs(cmd.PermissionIDs)
	permissions := make([]roleapplication.PermissionReference, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		permission, err := s.permissions.GetActiveByPermissionID(ctx, permissionID)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, *permission)
	}
	items, err := s.rolePermissions.Replace(ctx, cmd.RoleID, permissions)
	if err != nil {
		return nil, err
	}
	s.notifyPolicyChanged(ctx, "role_permissions_replaced")
	return &PermissionsResult{Items: items}, nil
}

// RemoveRolePermission 删除角色权限绑定；不存在由 store 映射为明确 not found 语义。
func (s *roleCommandService) RemoveRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error) {
	if err := s.rolePermissions.Remove(ctx, cmd.RoleID, cmd.PermissionID); err != nil {
		return nil, err
	}
	s.notifyPolicyChanged(ctx, "role_permission_removed")
	return s.listRolePermissions(ctx, cmd.RoleID)
}

func (s *roleCommandService) listUserRoles(ctx context.Context, userID uuid.UUID) (*RolesResult, error) {
	items, err := s.userRoles.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &RolesResult{Items: items}, nil
}

func (s *roleCommandService) listRolePermissions(ctx context.Context, roleID uuid.UUID) (*PermissionsResult, error) {
	items, err := s.rolePermissions.ListByRoleID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	return &PermissionsResult{Items: items}, nil
}

func uniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	result := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
