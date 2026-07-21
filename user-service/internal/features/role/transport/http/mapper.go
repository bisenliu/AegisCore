package rolehttp

import (
	"github.com/aegiscore/common/contract/pagination"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func toRoleResponse(role roledomain.Role) RoleResponse {
	return RoleResponse{RoleID: role.RoleID.String(), Name: role.Name, Description: role.Description, Active: role.Active, System: role.IsSystem, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}
}

func toRoleListResponse(result *rolequery.ListRolesResult) pagination.PaginatedData[RoleResponse] {
	items := toRoleResponses(result.Items)
	return pagination.NewPaginatedData(items, pagination.NewPagination(result.PageSize, result.NextCursor, result.HasNext))
}

func toRoleResponses(items []roledomain.Role) []RoleResponse {
	result := make([]RoleResponse, 0, len(items))
	for i := range items {
		result = append(result, toRoleResponse(items[i]))
	}
	return result
}

func toCommandRoleResponses(result *rolecommand.RolesResult) []RoleResponse {
	return toRoleResponses(result.Items)
}

func toPermissionResponse(permission roleapplication.PermissionReference) PermissionResponse {
	return PermissionResponse{PermissionID: permission.PermissionID.String(), Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
}

func toPermissionResponses(items []roleapplication.PermissionReference) []PermissionResponse {
	result := make([]PermissionResponse, 0, len(items))
	for i := range items {
		result = append(result, toPermissionResponse(items[i]))
	}
	return result
}
