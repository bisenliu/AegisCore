package command

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/features/permission/application/validators"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// CreatePermissionCommand 包含创建权限所需的应用层输入。
type CreatePermissionCommand struct {
	Name         string
	Description  string
	Module       string
	HTTPMethod   string
	PathTemplate string
	Active       *bool
	IsSystem     bool
}

// PermissionResult 是权限写侧用例的 transport-neutral 输出。
type PermissionResult struct {
	Permission permissiondomain.Permission
}

// PermissionCommandService 定义权限目录写侧用例。
type PermissionCommandService interface {
	CreatePermission(ctx context.Context, cmd CreatePermissionCommand) (*PermissionResult, error)
	UpdatePermission(ctx context.Context, cmd UpdatePermissionCommand) error
	EnablePermission(ctx context.Context, cmd SetPermissionActiveCommand) error
	DisablePermission(ctx context.Context, cmd SetPermissionActiveCommand) error
}

// CreatePermission 创建权限目录记录。
func (s *permissionCommandService) CreatePermission(ctx context.Context, cmd CreatePermissionCommand) (*PermissionResult, error) {
	name, description, module, identity, err := validators.NormalizePermissionFields(cmd.Name, cmd.Description, cmd.Module, cmd.HTTPMethod, cmd.PathTemplate)
	if err != nil {
		return nil, err
	}
	permissionID, err := runtimeid.NewUUID()
	if err != nil {
		logger.Error(ctx, "generate permission id failed", logger.StackTrace(zap.Error(err))...)
		return nil, fmt.Errorf("generate permission id: %w", err)
	}
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	created, err := s.store.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: name, Description: description, Module: module, HTTPMethod: identity.Method, PathTemplate: identity.PathTemplate, Active: active, IsSystem: cmd.IsSystem})
	if err != nil {
		if errors.Is(err, permissiondomain.ErrPermissionAlreadyExists) {
			logger.Warn(ctx, "create permission conflict", zap.String("http_method", identity.Method), zap.String("path_template", identity.PathTemplate))
			return nil, permissiondomain.ErrPermissionAlreadyExists
		}
		logger.Error(ctx, "create permission failed", logger.StackTrace(zap.String("permission_id", permissionID.String()), zap.Error(err))...)
		return nil, err
	}
	if err := s.notifyPolicyChanged(ctx, "permission_created"); err != nil {
		logger.Error(ctx, "refresh rbac policy after permission creation failed", logger.StackTrace(zap.String("permission_id", permissionID.String()), zap.Error(err))...)
	}
	return &PermissionResult{Permission: *created}, nil
}
