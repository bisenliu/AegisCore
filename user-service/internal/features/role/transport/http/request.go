package rolehttp

// RoleIDRequest 是通过外部 UUID 操作角色的 URI 绑定请求。
type RoleIDRequest struct {
	RoleID string `uri:"role_id" validate:"required,uuid" label:"角色ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// UserIDRequest 是通过外部 UUID 操作用户角色绑定的 URI 绑定请求。
type UserIDRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// UserRoleRequest 是通过外部 UUID 操作单个用户角色绑定的 URI 绑定请求。
type UserRoleRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	RoleID string `uri:"role_id" validate:"required,uuid" label:"角色ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// RolePermissionRequest 是通过外部 UUID 操作单个角色权限绑定的 URI 绑定请求。
type RolePermissionRequest struct {
	RoleID       string `uri:"role_id" validate:"required,uuid" label:"角色ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	PermissionID string `uri:"permission_id" validate:"required,uuid" label:"权限ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// ListRolesRequest 是分页角色列表和过滤条件的 query 绑定请求。
type ListRolesRequest struct {
	Cursor   string `query:"cursor" label:"分页游标" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	PageSize int    `query:"page_size" label:"每页数量" example:"20"`
	Limit    int    `query:"-"`
	Active   *bool  `query:"active" label:"是否启用" example:"true"`
	System   *bool  `query:"system" label:"是否系统角色" example:"true"`
}

// CreateRoleRequest 是创建角色的 JSON 请求体。
type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=128" label:"角色名称" example:"管理员"`
	Description string `json:"description" validate:"max=512" label:"角色说明" example:"管理后台角色"`
	Active      *bool  `json:"active,omitempty" label:"是否启用" example:"true"`
	System      bool   `json:"system" label:"是否系统角色" example:"false"`
}

// UpdateRoleRequest 是更新角色的 JSON 请求体。
type UpdateRoleRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=128" label:"角色名称" example:"管理员"`
	Description string `json:"description" validate:"max=512" label:"角色说明" example:"管理后台角色"`
	Active      bool   `json:"active" label:"是否启用" example:"true"`
}

// SetRoleStatusRequest 是启停角色的 JSON 请求体。
type SetRoleStatusRequest struct {
	Active bool `json:"active" label:"是否启用" example:"true"`
}

// RoleIDsRequest 是替换用户角色集合的 JSON 请求体。
type RoleIDsRequest struct {
	RoleIDs []string `json:"role_ids" validate:"required,dive,uuid" label:"角色ID列表"`
}

// RoleIDBodyRequest 是新增单个用户角色绑定的 JSON 请求体。
type RoleIDBodyRequest struct {
	RoleID string `json:"role_id" validate:"required,uuid" label:"角色ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// PermissionIDsRequest 是替换角色权限集合的 JSON 请求体。
type PermissionIDsRequest struct {
	PermissionIDs []string `json:"permission_ids" validate:"required,dive,uuid" label:"权限ID列表"`
}

// PermissionIDBodyRequest 是新增单个角色权限绑定的 JSON 请求体。
type PermissionIDBodyRequest struct {
	PermissionID string `json:"permission_id" validate:"required,uuid" label:"权限ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}
