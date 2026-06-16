package rolehttp

import "github.com/aegiscore/common/contract/pagination"

// RoleResponse 是角色公开响应。
type RoleResponse struct {
	RoleID      string `json:"role_id" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	Name        string `json:"name" example:"管理员"`
	Description string `json:"description" example:"管理后台角色"`
	Active      bool   `json:"active" example:"true"`
	System      bool   `json:"system" example:"false"`
	CreatedAt   int64  `json:"created_at" example:"1780288800000"`
	UpdatedAt   int64  `json:"updated_at" example:"1780288800000"`
}

// RoleListResponseDoc 描述分页角色列表 OpenAPI 响应载荷。
type RoleListResponseDoc struct {
	Items      []RoleResponse        `json:"items"`
	Pagination pagination.Pagination `json:"pagination"`
}

// PermissionResponse 是角色权限绑定中的权限公开响应。
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
