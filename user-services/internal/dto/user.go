package dto

import "github.com/aegiscore/common/response"

type GetUserRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

type ListUsersRequest struct {
	Page     int    `query:"page" label:"页码" example:"1"`
	PageSize int    `query:"page_size" label:"每页数量" example:"20"`
	Name     string `query:"name" label:"用户昵称" example:"Alice"`
	Username string `query:"username" label:"用户名" example:"alice"`
	Active   *bool  `query:"active" label:"是否启用" example:"true"`
}

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=128" label:"用户昵称" example:"Alice"`
	Username string `json:"username" validate:"required,min=1,max=255" label:"用户名" example:"alice"`
	Password string `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
	Active   *bool  `json:"active,omitempty" validate:"omitempty" label:"是否启用" example:"true"`
}

func (r *CreateUserRequest) SetDefaults() {
	if r.Active == nil {
		active := true
		r.Active = &active
	}
}

type UserResponse struct {
	UserID    string `json:"user_id" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	Name      string `json:"name" example:"Alice"`
	Username  string `json:"username" example:"alice"`
	Active    bool   `json:"active" example:"true"`
	CreatedAt int64  `json:"created_at" example:"1780288800000"`
	UpdatedAt int64  `json:"updated_at" example:"1780288800000"`
}

type UserListResponseDoc struct {
	Items      []UserResponse      `json:"items"`
	Pagination response.Pagination `json:"pagination"`
}
