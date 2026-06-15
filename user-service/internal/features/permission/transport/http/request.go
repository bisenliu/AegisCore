package permissionhttp

// PermissionIDRequest 是通过外部 UUID 操作权限的 URI 绑定请求。
type PermissionIDRequest struct {
	PermissionID string `uri:"permission_id" validate:"required,uuid" label:"权限ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// UserIDRequest 是通过外部 UUID 查询用户有效权限的 URI 绑定请求。
type UserIDRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// ListPermissionsRequest 是分页权限列表和过滤条件的 query 绑定请求。
type ListPermissionsRequest struct {
	Cursor     string `query:"cursor" label:"分页游标" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	PageSize   int    `query:"page_size" label:"每页数量" example:"20"`
	Limit      int    `query:"-"`
	Module     string `query:"module" label:"模块" example:"user"`
	HTTPMethod string `query:"http_method" label:"HTTP方法" example:"GET"`
	Active     *bool  `query:"active" label:"是否启用" example:"true"`
	System     *bool  `query:"system" label:"是否系统权限" example:"true"`
}

// CreatePermissionRequest 是创建权限目录记录的 JSON 请求体。
type CreatePermissionRequest struct {
	Name         string `json:"name" validate:"required,min=1,max=128" label:"权限名称" example:"查询用户列表"`
	Description  string `json:"description" validate:"max=512" label:"权限说明" example:"允许分页查询用户资料"`
	Module       string `json:"module" validate:"required,min=1,max=64" label:"模块" example:"user"`
	HTTPMethod   string `json:"http_method" validate:"required" label:"HTTP方法" example:"GET"`
	PathTemplate string `json:"path_template" validate:"required" label:"路径模板" example:"/api/v1/users"`
	Active       *bool  `json:"active,omitempty" label:"是否启用" example:"true"`
	System       bool   `json:"system" label:"是否系统权限" example:"false"`
}

// UpdatePermissionRequest 是更新权限目录记录的 JSON 请求体。
type UpdatePermissionRequest struct {
	Name         string `json:"name" validate:"required,min=1,max=128" label:"权限名称" example:"查询用户列表"`
	Description  string `json:"description" validate:"max=512" label:"权限说明" example:"允许分页查询用户资料"`
	Module       string `json:"module" validate:"required,min=1,max=64" label:"模块" example:"user"`
	HTTPMethod   string `json:"http_method" validate:"required" label:"HTTP方法" example:"GET"`
	PathTemplate string `json:"path_template" validate:"required" label:"路径模板" example:"/api/v1/users"`
	Active       bool   `json:"active" label:"是否启用" example:"true"`
}
