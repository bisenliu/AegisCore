package userhttp

import (
	"errors"

	"github.com/aegiscore/common/contract/response"
	userapi "github.com/aegiscore/user-services/internal/features/user/api"
	userapp "github.com/aegiscore/user-services/internal/features/user/app"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	"github.com/aegiscore/user-services/internal/messages"
)

func toUserResponse(result *userapp.UserResult) userapi.UserResponse {
	user := result.User
	return userapi.UserResponse{
		UserID:    user.UserID.String(),
		Nickname:  user.Nickname,
		Username:  user.Username,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toUserListResponse(result *userapp.ListUsersResult) response.PaginatedData[userapi.UserResponse] {
	items := make([]userapi.UserResponse, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, userapi.UserResponse{
			UserID:    result.Items[i].UserID.String(),
			Nickname:  result.Items[i].Nickname,
			Username:  result.Items[i].Username,
			Status:    result.Items[i].Status,
			CreatedAt: result.Items[i].CreatedAt,
			UpdatedAt: result.Items[i].UpdatedAt,
		})
	}
	return response.NewPaginatedData(items, response.NewPagination(result.Page, result.PageSize, result.Total))
}

func toUserHTTPError(err error) error {
	switch {
	case errors.Is(err, userdomain.ErrUserAlreadyExists):
		return response.ConflictError(messages.UserAlreadyExists)
	case errors.Is(err, userdomain.ErrUserNotFound):
		return response.NotFoundError(messages.UserNotFound)
	default:
		return response.FromError(err)
	}
}
