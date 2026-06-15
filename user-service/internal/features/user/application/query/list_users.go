package query

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

// ListUsersQuery 包含用户列表查询使用的规范化分页和过滤条件。
type ListUsersQuery struct {
	Cursor   *uuid.UUID
	PageSize int
	Limit    int
	Nickname string
	Username string
	Status   *identity.UserStatus
}

// ListUsersResult 是用户列表查询用例的 transport-neutral 分页输出。
type ListUsersResult struct {
	Items      []userdomain.User
	PageSize   int
	NextCursor string
	HasNext    bool
}

// ListUsers 使用规范化过滤条件返回分页用户资料列表。
func (s *userQueryService) ListUsers(ctx context.Context, query ListUsersQuery) (*ListUsersResult, error) {
	fields := []zap.Field{zap.Int("page_size", query.PageSize)}
	if query.Cursor != nil {
		fields = append(fields, zap.String("cursor", query.Cursor.String()))
	}
	logger.Info(ctx, "list users", fields...)
	users, hasNext, err := s.store.ListUsers(ctx, userapplication.ListUsersInput{
		AfterUserID: query.Cursor,
		Limit:       query.Limit,
		Nickname:    query.Nickname,
		Username:    query.Username,
		Status:      query.Status,
	})
	if err != nil {
		logger.Error(ctx, "list users failed", logger.StackTrace(append(fields, zap.Error(err))...)...)
		return nil, err
	}
	nextCursor := ""
	if hasNext && len(users) > 0 {
		nextCursor = users[len(users)-1].UserID.String()
	}
	return &ListUsersResult{Items: users, PageSize: query.PageSize, NextCursor: nextCursor, HasNext: hasNext}, nil
}
