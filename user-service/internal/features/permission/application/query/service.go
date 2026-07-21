package query

import (
	"context"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

// PermissionQueryService 定义权限目录读侧用例。
type PermissionQueryService interface {
	ListPermissions(ctx context.Context, query ListPermissionsQuery) (*ListPermissionsResult, error)
	ListUserEffectivePermissions(ctx context.Context, query UserEffectivePermissionsQuery) (*UserEffectivePermissionsResult, error)
}

type permissionQueryService struct {
	store permissionapplication.PermissionStore
}

// NewPermissionQueryService 根据仓储依赖构造权限读侧服务。
func NewPermissionQueryService(store permissionapplication.PermissionStore) PermissionQueryService {
	return &permissionQueryService{store: store}
}
