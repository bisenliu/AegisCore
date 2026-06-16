package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/aegiscore/common/contract/pagination"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

// ListRolesQuery 包含角色列表查询参数。
type ListRolesQuery struct {
	Cursor   *uuid.UUID
	PageSize int
	Limit    int
	Active   *bool
	IsSystem *bool
}

// GetRoleQuery 包含角色详情查询参数。
type GetRoleQuery struct {
	RoleID uuid.UUID
}

// UserRolesQuery 包含用户角色列表查询参数。
type UserRolesQuery struct {
	UserID uuid.UUID
}

// RolePermissionsQuery 包含角色权限列表查询参数。
type RolePermissionsQuery struct {
	RoleID uuid.UUID
}

// RoleResult 是角色详情查询用例的 transport-neutral 输出。
type RoleResult struct {
	Role roledomain.Role
}

// RolesResult 是角色集合查询用例的 transport-neutral 输出。
type RolesResult struct {
	Items []roledomain.Role
}

// ListRolesResult 是分页角色列表查询用例输出。
type ListRolesResult struct {
	Items      []roledomain.Role
	PageSize   int
	NextCursor string
	HasNext    bool
}

// PermissionsResult 是权限集合查询用例的 transport-neutral 输出。
type PermissionsResult struct {
	Items []roleapplication.PermissionReference
}

// ListRoles 返回分页角色列表。
func (s *roleQueryService) ListRoles(ctx context.Context, query ListRolesQuery) (*ListRolesResult, error) {
	pageSize := pagination.NormalizePageSize(query.PageSize)
	limit := query.Limit
	if limit <= 0 {
		limit = pageSize
	}
	items, hasNext, err := s.roles.List(ctx, roleapplication.ListRolesInput{AfterRoleID: query.Cursor, Limit: limit, Active: query.Active, IsSystem: query.IsSystem})
	if err != nil {
		return nil, err
	}
	nextCursor := ""
	if hasNext && len(items) > 0 {
		nextCursor = items[len(items)-1].RoleID.String()
	}
	return &ListRolesResult{Items: items, PageSize: pageSize, NextCursor: nextCursor, HasNext: hasNext}, nil
}

// GetRole 根据外部角色 ID 查询角色详情。
func (s *roleQueryService) GetRole(ctx context.Context, query GetRoleQuery) (*RoleResult, error) {
	role, err := s.roles.GetByRoleID(ctx, query.RoleID)
	if err != nil {
		return nil, err
	}
	return &RoleResult{Role: *role}, nil
}

// ListUserRoles 返回用户绑定的角色列表。
func (s *roleQueryService) ListUserRoles(ctx context.Context, query UserRolesQuery) (*RolesResult, error) {
	items, err := s.userRoles.ListByUserID(ctx, query.UserID)
	if err != nil {
		return nil, err
	}
	return &RolesResult{Items: items}, nil
}

// ListRolePermissions 返回角色绑定的权限列表。
func (s *roleQueryService) ListRolePermissions(ctx context.Context, query RolePermissionsQuery) (*PermissionsResult, error) {
	items, err := s.rolePermissions.ListByRoleID(ctx, query.RoleID)
	if err != nil {
		return nil, err
	}
	return &PermissionsResult{Items: items}, nil
}
