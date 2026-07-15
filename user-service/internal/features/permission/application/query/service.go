package query

import (
	"context"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

// PermissionQueryService 定义权限目录读侧用例。
type PermissionQueryService interface {
	ListPermissions(ctx context.Context, query ListPermissionsQuery) (*ListPermissionsResult, error)
	GetPermission(ctx context.Context, query GetPermissionQuery) (*PermissionResult, error)
	ListUserEffectivePermissions(ctx context.Context, query UserEffectivePermissionsQuery) (*UserEffectivePermissionsResult, error)
	GetRouteDiff(ctx context.Context) (*RouteDiffResult, error)
}

type permissionQueryService struct {
	scanner permissionapplication.RouteCatalogScanner
	store   permissionapplication.PermissionStore
	metrics permissionapplication.Metrics
}

// NewPermissionQueryService 根据仓储、路由扫描和指标依赖构造权限读侧服务。
func NewPermissionQueryService(store permissionapplication.PermissionStore, scanner permissionapplication.RouteCatalogScanner, metrics permissionapplication.Metrics) PermissionQueryService {
	return &permissionQueryService{store: store, scanner: scanner, metrics: metrics}
}
