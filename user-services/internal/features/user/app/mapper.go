package app

import (
	userapi "github.com/aegiscore/user-services/internal/features/user/api"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
)

func toUserResponse(user *userdomain.User) *userapi.UserResponse {
	return &userapi.UserResponse{
		UserID:    user.UserID.String(),
		Nickname:  user.Nickname,
		Username:  user.Username,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
