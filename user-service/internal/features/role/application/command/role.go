package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	"github.com/aegiscore/user-service/internal/features/role/application/validators"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

// RoleCommandService 定义角色管理写侧用例。
type RoleCommandService interface {
	CreateRole(ctx context.Context, cmd CreateRoleCommand) (*RoleResult, error)
	UpdateRole(ctx context.Context, cmd UpdateRoleCommand) (*RoleResult, error)
	SetRoleActive(ctx context.Context, cmd SetRoleActiveCommand) (*RoleResult, error)
	AddUserRole(ctx context.Context, cmd UserRoleCommand) (*RolesResult, error)
	ReplaceUserRoles(ctx context.Context, cmd ReplaceUserRolesCommand) (*RolesResult, error)
	RemoveUserRole(ctx context.Context, cmd UserRoleCommand) (*RolesResult, error)
	AddRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error)
	ReplaceRolePermissions(ctx context.Context, cmd ReplaceRolePermissionsCommand) (*PermissionsResult, error)
	RemoveRolePermission(ctx context.Context, cmd RolePermissionCommand) (*PermissionsResult, error)
}

// CreateRoleCommand 包含创建角色所需的应用层输入。
type CreateRoleCommand struct {
	Name        string
	Description string
	Active      *bool
	IsSystem    bool
}

// UpdateRoleCommand 包含更新角色所需的应用层输入。
type UpdateRoleCommand struct {
	RoleID      uuid.UUID
	Name        string
	Description string
	Active      bool
}

// SetRoleActiveCommand 包含启停角色所需的应用层输入。
type SetRoleActiveCommand struct {
	RoleID uuid.UUID
	Active bool
}

// RoleResult 是角色写侧用例的 transport-neutral 输出。
type RoleResult struct {
	Role roledomain.Role
}

// RolesResult 是角色集合写侧用例的 transport-neutral 输出。
type RolesResult struct {
	Items []roledomain.Role
}

// PermissionsResult 是权限集合写侧用例的 transport-neutral 输出。
type PermissionsResult struct {
	Items []roleapplication.PermissionReference
}

// CreateRole 创建角色记录。
func (s *roleCommandService) CreateRole(ctx context.Context, cmd CreateRoleCommand) (*RoleResult, error) {
	name, description, err := validators.NormalizeRoleFields(cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}
	roleID, err := uuid.NewV7()
	if err != nil {
		logger.Error(ctx, "generate role id failed", logger.StackTrace(zap.Error(err))...)
		return nil, fmt.Errorf("generate role id: %w", err)
	}
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	created, err := s.roles.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: name, Description: description, Active: active, IsSystem: cmd.IsSystem})
	if err != nil {
		logger.Error(ctx, "create role failed", logger.StackTrace(zap.String("role_id", roleID.String()), zap.Error(err))...)
		return nil, err
	}
	return &RoleResult{Role: *created}, nil
}

// UpdateRole 更新角色记录。
func (s *roleCommandService) UpdateRole(ctx context.Context, cmd UpdateRoleCommand) (*RoleResult, error) {
	name, description, err := validators.NormalizeRoleFields(cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}
	current, err := s.roles.GetByRoleID(ctx, cmd.RoleID)
	if err != nil {
		return nil, err
	}
	if err := current.ProtectSystemMutation(roledomain.RoleMutation{Name: name, Active: cmd.Active}); err != nil {
		return nil, err
	}
	updated, err := s.roles.Update(ctx, roleapplication.UpdateRoleInput{RoleID: cmd.RoleID, Name: name, Description: description, Active: cmd.Active})
	if err != nil {
		logger.Error(ctx, "update role failed", logger.StackTrace(zap.String("role_id", cmd.RoleID.String()), zap.Error(err))...)
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, "role_updated"); err != nil {
		logger.Error(ctx, "refresh rbac policy after role update failed", logger.StackTrace(zap.String("role_id", cmd.RoleID.String()), zap.Error(err))...)
	}
	return &RoleResult{Role: *updated}, nil
}

// SetRoleActive 启用或停用角色。
func (s *roleCommandService) SetRoleActive(ctx context.Context, cmd SetRoleActiveCommand) (*RoleResult, error) {
	current, err := s.roles.GetByRoleID(ctx, cmd.RoleID)
	if err != nil {
		return nil, err
	}
	if err := current.ProtectSystemMutation(roledomain.RoleMutation{Name: current.Name, Active: cmd.Active}); err != nil {
		return nil, err
	}
	updated, err := s.roles.SetActive(ctx, cmd.RoleID, cmd.Active)
	if err != nil {
		logger.Error(ctx, "set role active failed", logger.StackTrace(zap.String("role_id", cmd.RoleID.String()), zap.Bool("active", cmd.Active), zap.Error(err))...)
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, "role_active_changed"); err != nil {
		logger.Error(ctx, "refresh rbac policy after role active state change failed", logger.StackTrace(zap.String("role_id", cmd.RoleID.String()), zap.Bool("active", cmd.Active), zap.Error(err))...)
	}
	return &RoleResult{Role: *updated}, nil
}
