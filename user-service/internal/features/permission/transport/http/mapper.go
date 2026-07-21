package permissionhttp

import (
	"github.com/aegiscore/common/contract/pagination"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func toPermissionResponse(permission permissiondomain.Permission) PermissionResponse {
	return PermissionResponse{PermissionID: permission.PermissionID.String(), Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
}

func toPermissionListResponse(result *permissionquery.ListPermissionsResult) pagination.PaginatedData[PermissionResponse] {
	items := make([]PermissionResponse, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, toPermissionResponse(result.Items[i]))
	}
	return pagination.NewPaginatedData(items, pagination.NewPagination(result.PageSize, result.NextCursor, result.HasNext))
}

func toPermissionResponses(items []permissiondomain.Permission) []PermissionResponse {
	result := make([]PermissionResponse, 0, len(items))
	for i := range items {
		result = append(result, toPermissionResponse(items[i]))
	}
	return result
}
