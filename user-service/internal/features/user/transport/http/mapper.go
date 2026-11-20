package userhttp

import (
	"github.com/aegiscore/platform/contract/pagination"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
)

// toUserResponse 将用户信息转换为对外接口返回的格式，严格屏蔽密码哈希等敏感信息。
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

// toUserListResponse 将用户列表及分页信息打包为对外接口响应。
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
