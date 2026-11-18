package rolehttp

import (
	"github.com/aegiscore/common/contract/pagination"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

// toRoleResponse 将角色数据转换为对外接口返回的格式。
func toRoleResponse(role roledomain.Role) RoleResponse {
	return RoleResponse{RoleID: role.RoleID.String(), Name: role.Name, Description: role.Description, Active: role.Active, System: role.IsSystem, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}
}

// toRoleListResponse 将角色列表和分页信息打包为对外接口响应。
func toRoleListResponse(result *rolequery.ListRolesResult) pagination.PaginatedData[RoleResponse] {
	items := toRoleResponses(result.Items)
	return pagination.NewPaginatedData(items, pagination.NewPagination(result.PageSize, result.NextCursor, result.HasNext))
}

// toRoleResponses 批量将角色数据转换为对外接口返回格式。
func toRoleResponses(items []roledomain.Role) []RoleResponse {
	result := make([]RoleResponse, 0, len(items))
	for i := range items {
		result = append(result, toRoleResponse(items[i]))
	}
	return result
}

// toCommandRoleResponses 将角色分配或权限变更后的最新角色列表打包为接口响应。
func toCommandRoleResponses(result *rolecommand.RolesResult) []RoleResponse {
	return toRoleResponses(result.Items)
}

// toPermissionResponse 将角色下属的单项权限转换为对外接口返回格式。
func toPermissionResponse(permission roleapplication.PermissionReference) PermissionResponse {
	return PermissionResponse{PermissionID: permission.PermissionID.String(), Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
}

// toPermissionResponses 将角色下属的多项权限批量打包为对外接口返回格式。
func toPermissionResponses(items []roleapplication.PermissionReference) []PermissionResponse {
	result := make([]PermissionResponse, 0, len(items))
	for i := range items {
		result = append(result, toPermissionResponse(items[i]))
	}
	return result
}
