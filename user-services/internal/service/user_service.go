package service

import (
	"context"
	"errors"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/api/user"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/aegiscore/user-services/internal/validators"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UserService 定义暴露给 HTTP controller 的用户资料用例。
type UserService interface {
	CreateUser(ctx context.Context, req userapi.CreateUserRequest) (*userapi.UserResponse, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*userapi.UserResponse, error)
	ListUsers(ctx context.Context, req userapi.ListUsersRequest) (response.PaginatedData[userapi.UserResponse], error)
}

// UserProfileStore 定义用户资料 service 实际消费的持久化端口。
type UserProfileStore interface {
	Create(ctx context.Context, input CreateUserInput) (*domain.User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	ListUsers(ctx context.Context, input ListUsersInput) ([]domain.User, int, error)
}

// CreateUserInput 包含规范化后的用户创建数据和已哈希密码。
type CreateUserInput struct {
	Nickname     string
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       domain.UserStatus
}

// ListUsersInput 包含用户列表查询使用的规范化分页和过滤条件。
type ListUsersInput struct {
	Offset   int
	Limit    int
	Nickname string
	Username string
	Status   *domain.UserStatus
}

type userService struct {
	repo UserProfileStore
}

// NewUserService 根据仓储依赖构造用户资料服务。
func NewUserService(repo UserProfileStore) UserService {
	return &userService{repo: repo}
}

// CreateUser 创建新用户资料，哈希密码，并将 username 冲突映射为 API 错误。
func (s *userService) CreateUser(ctx context.Context, req userapi.CreateUserRequest) (*userapi.UserResponse, error) {
	status := domain.UserStatusNormal
	if req.Status != nil {
		// DTO 默认值通常会填充 status，此兜底保证 HTTP 之外的 service 调用同样安全。
		status = *req.Status
	}

	logger.Info(ctx, "create user", zap.String("username", req.Username), zap.Int64("status", int64(status)))
	passwordHash, err := password.Hash(req.Password)
	if err != nil {
		logger.Error(ctx, "hash user password failed", logger.StackTrace(zap.String("username", req.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, response.FromError(err)
	}

	userID, err := uuid.NewV7()
	if err != nil {
		logger.Error(ctx, "generate user id failed", logger.StackTrace(zap.String("username", req.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, response.FromError(err)
	}

	user, err := s.repo.Create(ctx, CreateUserInput{Nickname: req.Nickname, UserID: userID, Username: req.Username, PasswordHash: passwordHash, Status: status})
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			logger.Warn(ctx, "create user conflict", zap.String("username", req.Username), zap.Int64("status", int64(status)))
			return nil, response.ConflictError(messages.UserAlreadyExists)
		}
		logger.Error(ctx, "create user failed", logger.StackTrace(zap.String("username", req.Username), zap.String("user_id", userID.String()), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

// GetUserByID 按外部 UUID 返回公开用户资料。
func (s *userService) GetUserByID(ctx context.Context, userID uuid.UUID) (*userapi.UserResponse, error) {
	logger.Info(ctx, "query user profile", zap.String("user_id", userID.String()))
	user, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "query user profile not found", zap.String("user_id", userID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "query user profile failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

// ListUsers 使用规范化过滤条件返回分页用户资料列表。
func (s *userService) ListUsers(ctx context.Context, req userapi.ListUsersRequest) (response.PaginatedData[userapi.UserResponse], error) {
	validators.NormalizeListUsers(&req)

	logger.Info(ctx, "list users", zap.Int("page", req.Page), zap.Int("page_size", req.PageSize))
	users, total, err := s.repo.ListUsers(ctx, ListUsersInput{
		Offset:   req.Offset,
		Limit:    req.Limit,
		Nickname: req.Nickname,
		Username: req.Username,
		Status:   req.Status,
	})
	if err != nil {
		logger.Error(ctx, "list users failed", logger.StackTrace(zap.Int("page", req.Page), zap.Int("page_size", req.PageSize), zap.Error(err))...)
		return response.PaginatedData[userapi.UserResponse]{}, response.FromError(err)
	}

	items := make([]userapi.UserResponse, 0, len(users))
	for i := range users {
		items = append(items, *toUserResponse(&users[i]))
	}
	return response.NewPaginatedData(items, response.NewPagination(req.Page, req.PageSize, total)), nil
}

func toUserResponse(user *domain.User) *userapi.UserResponse {
	return &userapi.UserResponse{
		UserID:    user.UserID.String(),
		Nickname:  user.Nickname,
		Username:  user.Username,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
