package command

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
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
	role, err := s.roles.GetByRoleID(ctx, cmd.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, roledomain.ErrRoleNotFound
	}
	if !role.Active {
		return nil, roledomain.ErrRoleInactive
	}
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindUserRoleChanged, Reason: "user_role_added", UserID: cmd.UserID, RoleID: cmd.RoleID}
	write, err := s.userRoles.Add(ctx, cmd.UserID, cmd.RoleID, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyUserRoleChanged(ctx, write.Revision, change.Reason, cmd.UserID, cmd.RoleID); err != nil {
		logger.Error(ctx, "refresh rbac policy after user role add failed", logger.StackTrace(zap.String("user_id", cmd.UserID.String()), zap.String("role_id", cmd.RoleID.String()), zap.Error(err))...)
		return nil, err
	}
	return s.listUserRoles(ctx, cmd.UserID, write.Revision)
}

// ReplaceUserRoles 幂等替换用户的完整角色绑定集合。
// 输入 role_id 会先去重再整组替换；若替换已提交但 policy 通知失败，数据库绑定已生效，调用方需要依赖 watcher 补偿或重试恢复授权缓存一致性。
func (s *roleCommandService) ReplaceUserRoles(ctx context.Context, cmd ReplaceUserRolesCommand) (*RolesResult, error) {
	roleIDs := uniqueUUIDs(cmd.RoleIDs)
	roles, err := s.roles.GetByRoleIDs(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if !role.Active {
			return nil, roledomain.ErrRoleInactive
		}
	}
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindUserRoleChanged, Reason: "user_roles_replaced", UserID: cmd.UserID}
	write, err := s.userRoles.Replace(ctx, cmd.UserID, roleIDs, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyUserRoleChanged(ctx, write.Revision, change.Reason, cmd.UserID, uuid.Nil); err != nil {
		logger.Error(ctx, "refresh rbac policy after user roles replace failed", logger.StackTrace(zap.String("user_id", cmd.UserID.String()), zap.Error(err))...)
		return nil, err
	}
	return &RolesResult{Items: write.Items, Revision: write.Revision}, nil
}

// RemoveUserRole 删除用户角色绑定；不存在由 store 映射为明确 not found 语义。
func (s *roleCommandService) RemoveUserRole(ctx context.Context, cmd UserRoleCommand) (*RolesResult, error) {
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindUserRoleChanged, Reason: "user_role_removed", UserID: cmd.UserID, RoleID: cmd.RoleID}
	write, err := s.userRoles.Remove(ctx, cmd.UserID, cmd.RoleID, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyUserRoleChanged(ctx, write.Revision, change.Reason, cmd.UserID, cmd.RoleID); err != nil {
		logger.Error(ctx, "refresh rbac policy after user role remove failed", logger.StackTrace(zap.String("user_id", cmd.UserID.String()), zap.String("role_id", cmd.RoleID.String()), zap.Error(err))...)
		return nil, err
	}
	return s.listUserRoles(ctx, cmd.UserID, write.Revision)
}

// AddRolePermission 为角色新增权限绑定；重复绑定由 store 映射为明确冲突语义。
func (s *roleCommandService) AddRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error) {
	if _, err := s.roles.GetByRoleID(ctx, cmd.RoleID); err != nil {
		return nil, err
	}
	permission, err := s.permissions.GetByPermissionID(ctx, cmd.PermissionID)
	if err != nil {
		return nil, err
	}
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: "role_permission_added", RoleID: cmd.RoleID, PermissionID: cmd.PermissionID}
	write, err := s.rolePermissions.Add(ctx, cmd.RoleID, *permission, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, write.Revision, change.Reason); err != nil {
		logger.Error(ctx, "refresh rbac policy after role permission add failed", logger.StackTrace(zap.String("role_id", cmd.RoleID.String()), zap.String("permission_id", cmd.PermissionID.String()), zap.Error(err))...)
		return nil, err
	}
	return s.listRolePermissions(ctx, cmd.RoleID, write.Revision)
}

// ReplaceRolePermissions 幂等替换角色的完整权限绑定集合。
// 替换语义以去重后的完整权限集合为准；policy reload 失败会返回错误，但已提交的绑定不会回滚到旧授权集。
func (s *roleCommandService) ReplaceRolePermissions(ctx context.Context, cmd ReplaceRolePermissionsCommand) (*PermissionsResult, error) {
	if _, err := s.roles.GetByRoleID(ctx, cmd.RoleID); err != nil {
		return nil, err
	}
	permissionIDs := uniqueUUIDs(cmd.PermissionIDs)
	permissions := make([]roleapplication.PermissionReference, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		permission, err := s.permissions.GetByPermissionID(ctx, permissionID)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, *permission)
	}
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: "role_permissions_replaced", RoleID: cmd.RoleID}
	write, err := s.rolePermissions.Replace(ctx, cmd.RoleID, permissions, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, write.Revision, change.Reason); err != nil {
		logger.Error(ctx, "refresh rbac policy after role permissions replace failed", logger.StackTrace(zap.String("role_id", cmd.RoleID.String()), zap.Error(err))...)
		return nil, err
	}
	return &PermissionsResult{Items: write.Items, Revision: write.Revision}, nil
}

// RemoveRolePermission 删除角色权限绑定；不存在由 store 映射为明确 not found 语义。
func (s *roleCommandService) RemoveRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error) {
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: "role_permission_removed", RoleID: cmd.RoleID, PermissionID: cmd.PermissionID}
	write, err := s.rolePermissions.Remove(ctx, cmd.RoleID, cmd.PermissionID, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, write.Revision, change.Reason); err != nil {
		logger.Error(ctx, "refresh rbac policy after role permission remove failed", logger.StackTrace(zap.String("role_id", cmd.RoleID.String()), zap.String("permission_id", cmd.PermissionID.String()), zap.Error(err))...)
		return nil, err
	}
	return s.listRolePermissions(ctx, cmd.RoleID, write.Revision)
}

func (s *roleCommandService) listUserRoles(ctx context.Context, userID uuid.UUID, revision int64) (*RolesResult, error) {
	items, err := s.userRoles.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &RolesResult{Items: items, Revision: revision}, nil
}

func (s *roleCommandService) listRolePermissions(ctx context.Context, roleID uuid.UUID, revision int64) (*PermissionsResult, error) {
	items, err := s.rolePermissions.ListByRoleID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	return &PermissionsResult{Items: items, Revision: revision}, nil
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
