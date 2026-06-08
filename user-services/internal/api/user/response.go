package userapi

import "github.com/aegiscore/user-services/internal/domain"

// UserResponse 是不包含凭证字段的公开用户资料响应。
type UserResponse struct {
	UserID    string            `json:"user_id" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
	Nickname  string            `json:"nickname" example:"Alice"`
	Username  string            `json:"username" example:"alice"`
	Status    domain.UserStatus `json:"status" example:"100"`
	CreatedAt int64             `json:"created_at" example:"1780288800000"`
	UpdatedAt int64             `json:"updated_at" example:"1780288800000"`
}
