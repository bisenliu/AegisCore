package permissionhttp

import (
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// toPermissionResponse 将权限信息转换为对外接口返回的数据格式。
func toPermissionResponse(permission permissiondomain.Permission) PermissionResponse {
	return PermissionResponse{PermissionID: permission.PermissionID.String(), Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
}

// toPermissionListResponse 将权限列表及分页信息打包为对外接口响应。
func toPermissionListResponse(result *permissionquery.ListPermissionsResult) PermissionListResponseDoc {
	items := make([]PermissionResponse, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, toPermissionResponse(result.Items[i]))
	}
	return PermissionListResponseDoc{Items: items}
}

// toPermissionResponses 将多条权限记录批量打包为对外接口返回的数据格式。
func toPermissionResponses(items []permissiondomain.Permission) []PermissionResponse {
	result := make([]PermissionResponse, 0, len(items))
	for i := range items {
		result = append(result, toPermissionResponse(items[i]))
	}
	return result
}
