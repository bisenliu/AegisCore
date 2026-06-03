package dto

import (
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/internal/domain"
)

type GetUserRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

type ListUsersRequest struct {
	Page     int                `query:"page" label:"页码" example:"1"`
	PageSize int                `query:"page_size" label:"每页数量" example:"20"`
	Nickname string             `query:"nickname" label:"用户昵称" example:"Alice"`
	Username string             `query:"username" label:"用户名" example:"alice"`
	Status   *domain.UserStatus `query:"status" validate:"omitempty,enum" label:"用户状态" example:"100"`
}

type CreateUserRequest struct {
	Nickname string             `json:"nickname" validate:"required,min=1,max=128" label:"用户昵称" example:"Alice"`
	Username string             `json:"username" validate:"required,min=1,max=255" label:"用户名" example:"alice"`
	Password string             `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
	Status   *domain.UserStatus `json:"status,omitempty" validate:"omitempty,enum" label:"用户状态" example:"100"`
}

func (r *CreateUserRequest) SetDefaults() {
	if r.Status == nil {
		status := domain.UserStatusNormal
		r.Status = &status
	}
}

type UserResponse struct {
	UserID    string            `json:"user_id" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	Nickname  string            `json:"nickname" example:"Alice"`
	Username  string            `json:"username" example:"alice"`
	Status    domain.UserStatus `json:"status" example:"100"`
	CreatedAt int64             `json:"created_at" example:"1780288800000"`
	UpdatedAt int64             `json:"updated_at" example:"1780288800000"`
}

type UserListResponseDoc struct {
	Items      []UserResponse      `json:"items"`
	Pagination response.Pagination `json:"pagination"`
}
