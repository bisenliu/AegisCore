package userapi

import userdomain "github.com/aegiscore/user-service/internal/features/user/domain"

// GetUserRequest 是通过外部 UUID 查询用户的 URI 绑定请求。
type GetUserRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// ListUsersRequest 是分页用户列表和过滤条件的 query 绑定请求。
type ListUsersRequest struct {
	Cursor   string                 `query:"cursor" label:"分页游标" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	PageSize int                    `query:"page_size" label:"每页数量" example:"20"`
	Limit    int                    `query:"-"`
	Nickname string                 `query:"nickname" label:"用户昵称" example:"Alice"`
	Username string                 `query:"username" label:"用户名" example:"alice"`
	Status   *userdomain.UserStatus `query:"status" validate:"omitempty,enum" label:"用户状态" example:"100"`
}

// CreateUserRequest 是创建用户资料和凭证的 JSON 请求体。
type CreateUserRequest struct {
	Nickname string                 `json:"nickname" validate:"required,min=1,max=128" label:"用户昵称" example:"Alice"`
	Username string                 `json:"username" validate:"required,min=1,max=255" label:"用户名" example:"alice"`
	Password string                 `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
	Status   *userdomain.UserStatus `json:"status,omitempty" validate:"omitempty,enum" label:"用户状态" example:"100"`
}

// SetDefaults 在校验前将缺省用户状态设置为领域默认正常状态。
func (r *CreateUserRequest) SetDefaults() {
	if r.Status == nil {
		status := userdomain.UserStatusNormal
		r.Status = &status
	}
}
