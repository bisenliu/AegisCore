package dto

import "time"

type GetUserRequest struct {
	ID int64 `uri:"id" validate:"required,gt=0" label:"用户ID" example:"123"`
}

type CreateUserRequest struct {
	Name   string `json:"name" validate:"required,min=1,max=128" label:"用户名" example:"Alice"`
	Email  string `json:"email" validate:"required,email,max=255" label:"邮箱" example:"alice@example.com"`
	Active *bool  `json:"active,omitempty" validate:"omitempty" label:"是否启用" example:"true"`
}

func (r *CreateUserRequest) SetDefaults() {
	if r.Active == nil {
		active := true
		r.Active = &active
	}
}

type UserResponse struct {
	ID        int64     `json:"id" example:"123"`
	Name      string    `json:"name" example:"Alice"`
	Email     string    `json:"email" example:"alice@example.com"`
	Active    bool      `json:"active" example:"true"`
	CreatedAt time.Time `json:"created_at" example:"2026-05-29T10:00:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2026-05-29T10:00:00Z"`
}
