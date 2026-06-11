package userhttp

import (
	"strings"

	"github.com/aegiscore/common/contract/response"
	userapi "github.com/aegiscore/user-service/internal/features/user/api"
	"github.com/aegiscore/user-service/internal/messages"
	"github.com/google/uuid"
)

// NormalizeCreateUser 在 service 处理前裁剪用户创建输入，并将 username 转为小写。
func NormalizeCreateUser(req *userapi.CreateUserRequest) error {
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	req.Password = strings.TrimSpace(req.Password)
	if req.Nickname == "" || req.Username == "" {
		return response.ValidationFailedError(messages.InvalidUsername)
	}
	if req.Password == "" {
		return response.ValidationFailedError(messages.InvalidPassword)
	}
	return nil
}

// NormalizeListUsers 应用分页默认值并裁剪用户列表过滤条件。
func NormalizeListUsers(req *userapi.ListUsersRequest) {
	req.Cursor = strings.TrimSpace(req.Cursor)
	req.PageSize = response.NormalizePageSize(req.PageSize)
	req.Limit = req.PageSize
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Username = strings.TrimSpace(req.Username)
}

// ParseListCursor 将列表 cursor 转换为 UUID；空 cursor 表示第一页。
func ParseListCursor(req userapi.ListUsersRequest) (*uuid.UUID, error) {
	if req.Cursor == "" {
		return nil, nil
	}
	cursor, err := uuid.Parse(req.Cursor)
	if err != nil {
		return nil, response.BadRequestError(messages.InvalidUserID)
	}
	return &cursor, nil
}

// ParseUserID 将 URI user ID 转换为 UUID，并将无效输入映射为 API bad request。
func ParseUserID(req userapi.GetUserRequest) (uuid.UUID, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return uuid.Nil, response.BadRequestError(messages.InvalidUserID)
	}
	return userID, nil
}
