package query

import (
	"context"

	"github.com/google/uuid"

	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// UserEffectivePermissionsQuery 包含用户有效权限查询参数。
type UserEffectivePermissionsQuery struct {
	UserID uuid.UUID
}

// UserEffectivePermissionsResult 是用户有效权限查询输出。
type UserEffectivePermissionsResult struct {
	Items []permissiondomain.Permission
}

// ListUserEffectivePermissions 查询用户当前有效权限。
func (s *permissionQueryService) ListUserEffectivePermissions(ctx context.Context, query UserEffectivePermissionsQuery) (*UserEffectivePermissionsResult, error) {
	items, err := s.store.ListEffectiveByUserID(ctx, query.UserID)
	if err != nil {
		return nil, err
	}
	return &UserEffectivePermissionsResult{Items: items}, nil
}
