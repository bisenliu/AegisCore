package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
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
func (s *service) CreateUser(ctx context.Context, cmd CreateUserCommand) (*UserResult, error) {
	status := userdomain.UserStatusNormal
	if cmd.Status != nil {
		status = *cmd.Status
	}

	logger.Info(ctx, "create user", zap.String("username", cmd.Username), zap.Int64("status", int64(status)))
	passwordHash, err := password.HashContext(ctx, cmd.Password)
	if err != nil {
		logger.Error(ctx, "hash user password failed", logger.StackTrace(zap.String("username", cmd.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, fmt.Errorf("hash user password: %w", err)
	}

	userID, err := uuid.NewV7()
	if err != nil {
		logger.Error(ctx, "generate user id failed", logger.StackTrace(zap.String("username", cmd.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, fmt.Errorf("generate user id: %w", err)
	}

	user, err := s.store.Create(ctx, CreateUserInput{Nickname: cmd.Nickname, UserID: userID, Username: cmd.Username, PasswordHash: passwordHash, Status: status})
	if err != nil {
		if errors.Is(err, userdomain.ErrUserAlreadyExists) {
			logger.Warn(ctx, "create user conflict", zap.String("username", cmd.Username), zap.Int64("status", int64(status)))
			return nil, userdomain.ErrUserAlreadyExists
		}
		logger.Error(ctx, "create user failed", logger.StackTrace(zap.String("username", cmd.Username), zap.String("user_id", userID.String()), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, err
	}
	return &UserResult{User: *user}, nil
}

// GetUserByID 按外部 UUID 返回公开用户资料。
func (s *service) GetUserByID(ctx context.Context, userID uuid.UUID) (*UserResult, error) {
	logger.Info(ctx, "query user profile", zap.String("user_id", userID.String()))
	user, err := s.store.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "query user profile not found", zap.String("user_id", userID.String()))
			return nil, userdomain.ErrUserNotFound
		}
		logger.Error(ctx, "query user profile failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, err
	}
	return &UserResult{User: *user}, nil
}

// ListUsers 使用规范化过滤条件返回分页用户资料列表。
func (s *service) ListUsers(ctx context.Context, query ListUsersQuery) (*ListUsersResult, error) {
	fields := []zap.Field{zap.Int("page_size", query.PageSize)}
	if query.Cursor != nil {
		fields = append(fields, zap.String("cursor", query.Cursor.String()))
	}
	logger.Info(ctx, "list users", fields...)
	users, hasNext, err := s.store.ListUsers(ctx, ListUsersInput{
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
