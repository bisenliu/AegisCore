package application

import (
	"context"

	"github.com/google/uuid"

	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// PermissionStore 定义权限目录 use case 实际消费的持久化端口。
type PermissionStore interface {
	Create(ctx context.Context, input CreatePermissionInput) (*permissiondomain.Permission, error)
	GetByPermissionID(ctx context.Context, permissionID uuid.UUID) (*permissiondomain.Permission, error)
	List(ctx context.Context, input ListPermissionsInput) ([]permissiondomain.Permission, bool, error)
	ListAll(ctx context.Context) ([]permissiondomain.Permission, error)
	ListEffectiveByUserID(ctx context.Context, userID uuid.UUID) ([]permissiondomain.Permission, error)
	Update(ctx context.Context, input UpdatePermissionInput) error
	SetActive(ctx context.Context, permissionID uuid.UUID, active bool) error
}

// SeedPermissionStore 定义 RBAC seed 消费的权限持久化端口。
type SeedPermissionStore interface {
	UpsertSystemPermission(ctx context.Context, input SeedPermissionInput) (*permissiondomain.Permission, bool, error)
}

// RouteCatalogScanner 定义权限目录对已注册 HTTP 路由的只读扫描端口。
type RouteCatalogScanner interface {
	DiscoverRoutes(ctx context.Context) ([]DiscoveredRoute, error)
}

// CreatePermissionInput 包含规范化后的权限创建数据。
type CreatePermissionInput struct {
	PermissionID uuid.UUID
	Name         string
	Description  string
	Module       string
	HTTPMethod   string
	PathTemplate string
	Active       bool
	IsSystem     bool
}

// SeedPermissionInput 包含系统权限 seed 规范化后的写入数据。
type SeedPermissionInput struct {
	PermissionID     uuid.UUID
	Name             string
	Description      string
	Module           string
	HTTPMethod       string
	PathTemplate     string
	Active           bool
	IsSystem         bool
	ReactivateSystem bool
}

// UpdatePermissionInput 包含规范化后的权限更新数据。
type UpdatePermissionInput struct {
	PermissionID uuid.UUID
	Name         string
	Description  string
	Module       string
	HTTPMethod   string
	PathTemplate string
	Active       bool
}

// ListPermissionsInput 包含权限目录列表查询过滤和分页条件。
type ListPermissionsInput struct {
	AfterPermissionID *uuid.UUID
	Limit             int
	Module            string
	HTTPMethod        string
	Active            *bool
	IsSystem          *bool
}

// DiscoveredRoute 是 route scanner 返回的 transport-neutral 路由条目。
type DiscoveredRoute struct {
	Method string
	Path   string
}
