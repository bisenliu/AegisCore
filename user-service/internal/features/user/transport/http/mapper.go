package userhttp

import (
	"github.com/aegiscore/common/contract/pagination"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
)

func toUserResponse(user userdomain.User) UserResponse {
	return UserResponse{
		UserID:    user.UserID.String(),
		Nickname:  user.Nickname,
		Username:  user.Username,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toUserListResponse(result *userquery.ListUsersResult) pagination.PaginatedData[UserResponse] {
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
