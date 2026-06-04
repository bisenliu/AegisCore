package service

import (
	"context"
	"errors"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/errmsg"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserService interface {
	CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*dto.UserResponse, error)
	ListUsers(ctx context.Context, req dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	status := domain.UserStatusNormal
	if req.Status != nil {
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

	user, err := s.repo.Create(ctx, repository.CreateUserInput{Nickname: req.Nickname, UserID: userID, Username: req.Username, PasswordHash: passwordHash, Status: status})
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			logger.Warn(ctx, "create user conflict", zap.String("username", req.Username), zap.Int64("status", int64(status)))
			return nil, response.ConflictError(errmsg.MsgUserAlreadyExists)
		}
		logger.Error(ctx, "create user failed", logger.StackTrace(zap.String("username", req.Username), zap.String("user_id", userID.String()), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) GetUserByID(ctx context.Context, userID uuid.UUID) (*dto.UserResponse, error) {
	logger.Info(ctx, "query user profile", zap.String("user_id", userID.String()))
	user, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "query user profile not found", zap.String("user_id", userID.String()))
			return nil, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "query user profile failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) ListUsers(ctx context.Context, req dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error) {
	logger.Info(ctx, "list users", zap.Int("page", req.Page), zap.Int("page_size", req.PageSize))
	users, total, err := s.repo.ListUsers(ctx, repository.ListUsersInput{
		Offset:   req.Offset,
		Limit:    req.Limit,
		Nickname: req.Nickname,
		Username: req.Username,
		Status:   req.Status,
	})
	if err != nil {
		logger.Error(ctx, "list users failed", logger.StackTrace(zap.Int("page", req.Page), zap.Int("page_size", req.PageSize), zap.Error(err))...)
		return response.PaginatedData[dto.UserResponse]{}, response.FromError(err)
	}

	items := make([]dto.UserResponse, 0, len(users))
	for i := range users {
		items = append(items, *toUserResponse(&users[i]))
	}
	return response.NewPaginatedData(items, response.NewPagination(req.Page, req.PageSize, total)), nil
}

func toUserResponse(user *domain.User) *dto.UserResponse {
	return &dto.UserResponse{
		UserID:    user.UserID.String(),
		Nickname:  user.Nickname,
		Username:  user.Username,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
