package userhttp

import (
	"strings"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	"github.com/aegiscore/user-service/internal/messages"
)

// prepareListUsersQuery 将用户列表 HTTP 请求转换为应用层查询。
func prepareListUsersQuery(req ListUsersRequest) (userquery.ListUsersQuery, error) {
	cursorText := strings.TrimSpace(req.Cursor)
	var cursor *uuid.UUID
	if cursorText != "" {
		parsed, err := uuid.Parse(cursorText)
		if err != nil {
			return userquery.ListUsersQuery{}, contracterrors.BadRequestError(messages.InvalidUserID)
		}
		cursor = &parsed
	}
	pageSize := pagination.NormalizePageSize(req.PageSize)
	return userquery.ListUsersQuery{
		Cursor:   cursor,
		PageSize: pageSize,
		Limit:    pageSize,
		Nickname: strings.TrimSpace(req.Nickname),
		Username: strings.TrimSpace(req.Username),
		Status:   req.Status,
	}, nil
}

// prepareCreateUserCommand 将创建用户 HTTP 请求转换为应用层命令。
func prepareCreateUserCommand(req CreateUserRequest) (usercommand.CreateUserCommand, error) {
	nickname := strings.TrimSpace(req.Nickname)
	username := strings.ToLower(strings.TrimSpace(req.Username))
	password := strings.TrimSpace(req.Password)
	if nickname == "" || username == "" {
		return usercommand.CreateUserCommand{}, contracterrors.ValidationFailedError(messages.InvalidUsername)
	}
	if password == "" {
		return usercommand.CreateUserCommand{}, contracterrors.ValidationFailedError(messages.InvalidPassword)
	}
	return usercommand.CreateUserCommand{Nickname: nickname, Username: username, Password: password, Status: req.Status}, nil
}

// prepareGetUserByIDQuery 将用户 ID URI 请求转换为应用层查询。
func prepareGetUserByIDQuery(req GetUserRequest) (userquery.GetUserByIDQuery, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return userquery.GetUserByIDQuery{}, contracterrors.BadRequestError(messages.InvalidUserID)
	}
	return userquery.GetUserByIDQuery{UserID: userID}, nil
}
