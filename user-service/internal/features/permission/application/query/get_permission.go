package query

import (
	"context"

	"github.com/google/uuid"

	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// GetPermissionQuery 包含权限详情查询参数。
type GetPermissionQuery struct {
	PermissionID uuid.UUID
}

// PermissionResult 是权限详情查询用例的 transport-neutral 输出。
type PermissionResult struct {
	Permission permissiondomain.Permission
}

// GetPermission 根据外部权限 ID 查询权限详情。
func (s *permissionQueryService) GetPermission(ctx context.Context, query GetPermissionQuery) (*PermissionResult, error) {
	permission, err := s.store.GetByPermissionID(ctx, query.PermissionID)
	if err != nil {
		return nil, err
	}
	return &PermissionResult{Permission: *permission}, nil
}
