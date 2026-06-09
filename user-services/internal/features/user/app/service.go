package app

import (
	"context"
	"errors"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	userapi "github.com/aegiscore/user-services/internal/features/user/api"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type service struct {
	store UserProfileStore
}

// NewUserService 根据仓储依赖构造用户资料服务。
func NewUserService(store UserProfileStore) UserService {
	return &service{store: store}
}

// CreateUser 创建新用户资料，哈希密码，并将 username 冲突映射为 API 错误。
func (s *service) CreateUser(ctx context.Context, cmd CreateUserCommand) (*userapi.UserResponse, error) {
	status := userdomain.UserStatusNormal
	if cmd.Status != nil {
		status = *cmd.Status
	}

	logger.Info(ctx, "create user", zap.String("username", cmd.Username), zap.Int64("status", int64(status)))
	passwordHash, err := password.HashContext(ctx, cmd.Password)
	if err != nil {
		logger.Error(ctx, "hash user password failed", logger.StackTrace(zap.String("username", cmd.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, response.FromError(err)
	}

	userID, err := uuid.NewV7()
	if err != nil {
		logger.Error(ctx, "generate user id failed", logger.StackTrace(zap.String("username", cmd.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, response.FromError(err)
	}

	user, err := s.store.Create(ctx, CreateUserInput{Nickname: cmd.Nickname, UserID: userID, Username: cmd.Username, PasswordHash: passwordHash, Status: status})
	if err != nil {
		if errors.Is(err, userdomain.ErrUserAlreadyExists) {
			logger.Warn(ctx, "create user conflict", zap.String("username", cmd.Username), zap.Int64("status", int64(status)))
			return nil, response.ConflictError(messages.UserAlreadyExists)
		}
		logger.Error(ctx, "create user failed", logger.StackTrace(zap.String("username", cmd.Username), zap.String("user_id", userID.String()), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

// GetUserByID 按外部 UUID 返回公开用户资料。
func (s *service) GetUserByID(ctx context.Context, userID uuid.UUID) (*userapi.UserResponse, error) {
	logger.Info(ctx, "query user profile", zap.String("user_id", userID.String()))
	user, err := s.store.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "query user profile not found", zap.String("user_id", userID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "query user profile failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

// ListUsers 使用规范化过滤条件返回分页用户资料列表。
func (s *service) ListUsers(ctx context.Context, query ListUsersQuery) (response.PaginatedData[userapi.UserResponse], error) {
	logger.Info(ctx, "list users", zap.Int("page", query.Page), zap.Int("page_size", query.PageSize))
	users, total, err := s.store.ListUsers(ctx, ListUsersInput{
		Offset:   query.Offset,
		Limit:    query.Limit,
		Nickname: query.Nickname,
		Username: query.Username,
		Status:   query.Status,
	})
	if err != nil {
		logger.Error(ctx, "list users failed", logger.StackTrace(zap.Int("page", query.Page), zap.Int("page_size", query.PageSize), zap.Error(err))...)
		return response.PaginatedData[userapi.UserResponse]{}, response.FromError(err)
	}

	items := make([]userapi.UserResponse, 0, len(users))
	for i := range users {
		items = append(items, *toUserResponse(&users[i]))
	}
	result := response.NewPaginatedData(items, response.NewPagination(query.Page, query.PageSize, total))
	return result, nil
}
