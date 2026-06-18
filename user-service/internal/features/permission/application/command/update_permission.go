package command

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/features/permission/application/validators"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// UpdatePermissionCommand 包含更新权限所需的应用层输入。
type UpdatePermissionCommand struct {
	PermissionID uuid.UUID
	Name         string
	Description  string
	Module       string
	HTTPMethod   string
	PathTemplate string
	Active       bool
}

// UpdatePermission 更新权限元数据并保护系统权限身份。
func (s *permissionCommandService) UpdatePermission(ctx context.Context, cmd UpdatePermissionCommand) (*PermissionResult, error) {
	name, description, module, identity, err := validators.NormalizePermissionFields(cmd.Name, cmd.Description, cmd.Module, cmd.HTTPMethod, cmd.PathTemplate)
	if err != nil {
		return nil, err
	}
	current, err := s.store.GetByPermissionID(ctx, cmd.PermissionID)
	if err != nil {
		return nil, err
	}
	if err := current.ProtectSystemIdentity(identity); err != nil {
		logger.Warn(ctx, "reject system permission identity update", zap.String("permission_id", cmd.PermissionID.String()))
		return nil, err
	}
	updated, err := s.store.Update(ctx, permissionapplication.UpdatePermissionInput{PermissionID: cmd.PermissionID, Name: name, Description: description, Module: module, HTTPMethod: identity.Method, PathTemplate: identity.PathTemplate, Active: cmd.Active})
	if err != nil {
		if errors.Is(err, permissiondomain.ErrPermissionAlreadyExists) {
			return nil, permissiondomain.ErrPermissionAlreadyExists
		}
		logger.Error(ctx, "update permission failed", logger.StackTrace(zap.String("permission_id", cmd.PermissionID.String()), zap.Error(err))...)
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, "permission_updated"); err != nil {
		logger.Error(ctx, "refresh rbac policy after permission update failed", logger.StackTrace(zap.String("permission_id", cmd.PermissionID.String()), zap.Error(err))...)
		return nil, err
	}
	return &PermissionResult{Permission: *updated}, nil
}
