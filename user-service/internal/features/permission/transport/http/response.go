package permissionhttp

import "github.com/aegiscore/common/contract/pagination"

// PermissionResponse 是权限目录公开响应。
type PermissionResponse struct {
	PermissionID string `json:"permission_id" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	Name         string `json:"name" example:"查询用户列表"`
	Description  string `json:"description" example:"允许分页查询用户资料"`
	Module       string `json:"module" example:"user"`
	HTTPMethod   string `json:"http_method" example:"GET"`
	PathTemplate string `json:"path_template" example:"/api/v1/users"`
	Active       bool   `json:"active" example:"true"`
	System       bool   `json:"system" example:"true"`
	CreatedAt    int64  `json:"created_at" example:"1780288800000"`
	UpdatedAt    int64  `json:"updated_at" example:"1780288800000"`
}

// PermissionListResponseDoc 描述分页权限列表 OpenAPI 响应载荷。
type PermissionListResponseDoc struct {
	Items      []PermissionResponse  `json:"items"`
	Pagination pagination.Pagination `json:"pagination"`
}

// DiscoveredRouteResponse 是 route diff 中缺失路由响应。
type DiscoveredRouteResponse struct {
	HTTPMethod string `json:"http_method" example:"GET"`
	Path       string `json:"path" example:"/api/v1/users"`
}

// RouteDiffResponse 是权限目录与路由发现的差异响应。
type RouteDiffResponse struct {
	MissingInPermissions []DiscoveredRouteResponse `json:"missing_in_permissions"`
	StalePermissions     []PermissionResponse      `json:"stale_permissions"`
}
