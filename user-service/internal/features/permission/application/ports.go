package application

import (
	"context"

	"github.com/google/uuid"

	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// PermissionStore 定义权限目录 use case 实际消费的持久化端口。
type PermissionStore interface {
	GetByPermissionID(ctx context.Context, permissionID uuid.UUID) (*permissiondomain.Permission, error)
	List(ctx context.Context, input ListPermissionsInput) ([]permissiondomain.Permission, error)
	ListEffectiveByUserID(ctx context.Context, userID uuid.UUID) ([]permissiondomain.Permission, error)
}

// SeedPermissionStore 定义 RBAC seed 消费的权限持久化端口。
type SeedPermissionStore interface {
	UpsertPermission(ctx context.Context, input SeedPermissionInput) (*permissiondomain.Permission, bool, error)
}

// SeedPermissionInput 包含系统权限 seed 规范化后的写入数据。
type SeedPermissionInput struct {
	PermissionID uuid.UUID
	Name         string
	Description  string
	Module       string
	HTTPMethod   string
	PathTemplate string
}

// ListPermissionsInput 包含权限目录列表查询过滤条件。
type ListPermissionsInput struct {
	Module     string
	HTTPMethod string
}
