package application

import (
	"context"

	"github.com/google/uuid"

	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

// RoleStore 定义角色生命周期 use case 实际消费的角色持久化端口。
type RoleStore interface {
	Create(ctx context.Context, input CreateRoleInput) (*roledomain.Role, error)
	GetByRoleID(ctx context.Context, roleID uuid.UUID) (*roledomain.Role, error)
	GetByRoleIDs(ctx context.Context, roleIDs []uuid.UUID) ([]roledomain.Role, error)
	List(ctx context.Context, input ListRolesInput) ([]roledomain.Role, bool, error)
	Update(ctx context.Context, input UpdateRoleInput) error
	SetActive(ctx context.Context, roleID uuid.UUID, active bool) error
}

// SeedRoleStore 定义 RBAC seed 消费的角色持久化端口。
type SeedRoleStore interface {
	UpsertSystemRole(ctx context.Context, input SeedRoleInput) (*roledomain.Role, bool, error)
}

// UserRoleStore 定义用户角色绑定 use case 实际消费的持久化端口。
type UserRoleStore interface {
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]roledomain.Role, error)
	Add(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error
	Replace(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) ([]roledomain.Role, error)
	Remove(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error
}

// SeedUserRoleStore 定义 RBAC seed 消费的用户角色绑定端口。
type SeedUserRoleStore interface {
	AssignRole(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) (bool, error)
}

// RolePermissionStore 定义角色权限绑定 use case 实际消费的持久化端口。
type RolePermissionStore interface {
	ListByRoleID(ctx context.Context, roleID uuid.UUID) ([]PermissionReference, error)
	Add(ctx context.Context, roleID uuid.UUID, permission PermissionReference) error
	Replace(ctx context.Context, roleID uuid.UUID, permissions []PermissionReference) ([]PermissionReference, error)
	Remove(ctx context.Context, roleID uuid.UUID, permissionID uuid.UUID) error
}

// SeedRolePermissionStore 定义 RBAC seed 消费的系统角色权限绑定端口。
type SeedRolePermissionStore interface {
	EnsureSystemBindings(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, error)
	SyncSystemBindings(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, int, error)
}

// PermissionLookup 定义角色绑定权限前对权限目录的只读校验端口。
type PermissionLookup interface {
	GetActiveByPermissionID(ctx context.Context, permissionID uuid.UUID) (*PermissionReference, error)
}

// CreateRoleInput 包含规范化后的角色创建数据。
type CreateRoleInput struct {
	RoleID      uuid.UUID
	Name        string
	Description string
	Active      bool
	IsSystem    bool
}

// SeedRoleInput 包含系统角色 seed 规范化后的写入数据。
type SeedRoleInput struct {
	RoleID           uuid.UUID
	Name             string
	Description      string
	Active           bool
	IsSystem         bool
	ReactivateSystem bool
}

// UpdateRoleInput 包含规范化后的角色更新数据。
type UpdateRoleInput struct {
	RoleID      uuid.UUID
	Name        string
	Description string
	Active      bool
}

// ListRolesInput 包含角色列表查询过滤和分页条件。
type ListRolesInput struct {
	AfterRoleID *uuid.UUID
	Limit       int
	Active      *bool
	IsSystem    *bool
}

// PermissionReference 是角色 feature 对权限目录只读身份的最小视图。
type PermissionReference struct {
	ID           int64
	PermissionID uuid.UUID
	Name         string
	Description  string
	Module       string
	HTTPMethod   string
	PathTemplate string
	Active       bool
	IsSystem     bool
	CreatedAt    int64
	UpdatedAt    int64
}
