package command

import (
	"context"
	"uuid"

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

// AddUserRole 为用户绑定新的角色；若该角色已绑定过，则返回数据已存在的冲突错误。
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
		logger.Warn(ctx, "refresh rbac policy after user role add failed", zap.Int64("policy_revision", write.Revision), zap.String("user_id", cmd.UserID.String()), zap.String("role_id", cmd.RoleID.String()), zap.Error(err))
	}
	return &RolesResult{Items: write.Items, Revision: write.Revision}, nil
}

// ReplaceUserRoles 全量覆盖用户的角色绑定列表（多次提交相同内容结果一致）。
// 系统会先剔除重复的角色 ID 再执行整组更新；若数据库更新已成功但权限变更广播通知失败，权限数据仍以数据库为准，后台定时同步机制会自动校准权限缓存。
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
	if err := s.notifyUserRoleChanged(ctx, write.Revision, change.Reason, cmd.UserID, uuid.Nil()); err != nil {
		logger.Warn(ctx, "refresh rbac policy after user roles replace failed", zap.Int64("policy_revision", write.Revision), zap.String("user_id", cmd.UserID.String()), zap.Error(err))
	}
	return &RolesResult{Items: write.Items, Revision: write.Revision}, nil
}

// RemoveUserRole 解除用户的指定角色；若该绑定关系本就不存在，则返回未找到的错误。
func (s *roleCommandService) RemoveUserRole(ctx context.Context, cmd UserRoleCommand) (*RolesResult, error) {
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindUserRoleChanged, Reason: "user_role_removed", UserID: cmd.UserID, RoleID: cmd.RoleID}
	write, err := s.userRoles.Remove(ctx, cmd.UserID, cmd.RoleID, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyUserRoleChanged(ctx, write.Revision, change.Reason, cmd.UserID, cmd.RoleID); err != nil {
		logger.Warn(ctx, "refresh rbac policy after user role remove failed", zap.Int64("policy_revision", write.Revision), zap.String("user_id", cmd.UserID.String()), zap.String("role_id", cmd.RoleID.String()), zap.Error(err))
	}
	return &RolesResult{Items: write.Items, Revision: write.Revision}, nil
}

// AddRolePermission 为角色分配新的权限；若该角色已拥有该权限，则返回重复绑定的冲突错误。
func (s *roleCommandService) AddRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error) {
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
		logger.Warn(ctx, "refresh rbac policy after role permission add failed", zap.Int64("policy_revision", write.Revision), zap.String("role_id", cmd.RoleID.String()), zap.String("permission_id", cmd.PermissionID.String()), zap.Error(err))
	}
	return &PermissionsResult{Items: write.Items, Revision: write.Revision}, nil
}

// ReplaceRolePermissions 全量覆盖角色的权限列表（多次提交相同内容结果一致）。
// 系统会自动去重后整组更新；若更新提交后权限引擎热重载失败，系统会通过数据库版本号机制在后台定时自动校准恢复。
func (s *roleCommandService) ReplaceRolePermissions(ctx context.Context, cmd ReplaceRolePermissionsCommand) (*PermissionsResult, error) {
	permissionIDs := uniqueUUIDs(cmd.PermissionIDs)
	permissions, err := s.permissions.GetByPermissionIDs(ctx, permissionIDs)
	if err != nil {
		return nil, err
	}
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: "role_permissions_replaced", RoleID: cmd.RoleID}
	write, err := s.rolePermissions.Replace(ctx, cmd.RoleID, permissions, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, write.Revision, change.Reason); err != nil {
		logger.Warn(ctx, "refresh rbac policy after role permissions replace failed", zap.Int64("policy_revision", write.Revision), zap.String("role_id", cmd.RoleID.String()), zap.Error(err))
	}
	return &PermissionsResult{Items: write.Items, Revision: write.Revision}, nil
}

// RemoveRolePermission 移除角色的指定权限；若该角色原本就没有绑定该权限，则返回未找到的错误。
func (s *roleCommandService) RemoveRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error) {
	change := roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: "role_permission_removed", RoleID: cmd.RoleID, PermissionID: cmd.PermissionID}
	write, err := s.rolePermissions.Remove(ctx, cmd.RoleID, cmd.PermissionID, change)
	if err != nil {
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, write.Revision, change.Reason); err != nil {
		logger.Warn(ctx, "refresh rbac policy after role permission remove failed", zap.Int64("policy_revision", write.Revision), zap.String("role_id", cmd.RoleID.String()), zap.String("permission_id", cmd.PermissionID.String()), zap.Error(err))
	}
	return &PermissionsResult{Items: write.Items, Revision: write.Revision}, nil
}

// uniqueUUIDs 剔除列表中重复的编号，同时保持用户传入的先后顺序。
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
