package permissionhttp

// UserIDRequest 是通过外部 UUID 查询用户有效权限的 URI 绑定请求。
type UserIDRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// ListPermissionsRequest 是权限列表过滤条件的 query 绑定请求。
type ListPermissionsRequest struct {
	Module     string `query:"module" label:"模块" example:"user"`
	HTTPMethod string `query:"http_method" label:"HTTP方法" example:"GET"`
}
