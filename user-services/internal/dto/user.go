package dto

import "time"

type GetUserRequest struct {
	ID int64 `uri:"id" validate:"required,gt=0" label:"用户ID"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
