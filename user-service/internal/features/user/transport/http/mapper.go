package userhttp

import (
	"errors"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func toUserResponse(result *userapplication.UserResult) UserResponse {
	user := result.User
	return UserResponse{
		UserID:    user.UserID.String(),
		Nickname:  user.Nickname,
		Username:  user.Username,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toUserListResponse(result *userapplication.ListUsersResult) pagination.PaginatedData[UserResponse] {
	items := make([]UserResponse, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, UserResponse{
			UserID:    result.Items[i].UserID.String(),
			Nickname:  result.Items[i].Nickname,
			Username:  result.Items[i].Username,
			Status:    result.Items[i].Status,
			CreatedAt: result.Items[i].CreatedAt,
			UpdatedAt: result.Items[i].UpdatedAt,
		})
	}
	return pagination.NewPaginatedData(items, pagination.NewPagination(result.PageSize, result.NextCursor, result.HasNext))
}

func toUserHTTPError(err error) error {
	switch {
	case errors.Is(err, userdomain.ErrUserAlreadyExists):
		return contracterrors.ConflictError(messages.UserAlreadyExists)
	case errors.Is(err, userdomain.ErrUserNotFound):
		return contracterrors.NotFoundError(messages.UserNotFound)
	default:
		return contracterrors.FromError(err)
	}
}
