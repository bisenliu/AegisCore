package command

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
)

// SetPermissionActiveCommand 包含启停权限所需的应用层输入。
type SetPermissionActiveCommand struct {
	PermissionID uuid.UUID
}

// EnablePermission 启用权限目录记录。
func (s *permissionCommandService) EnablePermission(ctx context.Context, cmd SetPermissionActiveCommand) (*PermissionResult, error) {
	return s.setPermissionActive(ctx, cmd.PermissionID, true)
}

// DisablePermission 停用权限目录记录，不物理删除记录。
func (s *permissionCommandService) DisablePermission(ctx context.Context, cmd SetPermissionActiveCommand) (*PermissionResult, error) {
	return s.setPermissionActive(ctx, cmd.PermissionID, false)
}

func (s *permissionCommandService) setPermissionActive(ctx context.Context, permissionID uuid.UUID, active bool) (*PermissionResult, error) {
	updated, err := s.store.SetActive(ctx, permissionID, active)
	if err != nil {
		logger.Error(ctx, "set permission active failed", logger.StackTrace(zap.String("permission_id", permissionID.String()), zap.Bool("active", active), zap.Error(err))...)
		return nil, err
	}
	s.notifyPolicyChanged(ctx, "permission_active_changed")
	return &PermissionResult{Permission: *updated}, nil
}
