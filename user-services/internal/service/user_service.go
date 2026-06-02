package service

import (
	"context"
	"strings"

	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/apperror"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/aegiscore/user-services/internal/security"
	"go.uber.org/zap"
)

type UserService interface {
	CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error)
	GetUserByID(ctx context.Context, id int64) (*dto.UserResponse, error)
	ListUsers(ctx context.Context, req dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	if name == "" {
		return nil, response.ValidationFailedError(apperror.MsgInvalidUserName)
	}
	if password == "" {
		return nil, response.ValidationFailedError(apperror.MsgInvalidPassword)
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	logger.Info(ctx, "create user", zap.String("email", email))
	exists, err := s.repo.ExistsByEmail(ctx, email)
	if err != nil {
		logger.Error(ctx, "check user email failed", zap.String("email", email), zap.Error(err))
		return nil, response.FromError(err)
	}
	if exists {
		return nil, response.ConflictError(apperror.MsgUserAlreadyExists)
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		logger.Error(ctx, "hash user password failed", zap.String("email", email), zap.Error(err))
		return nil, response.FromError(err)
	}

	user, err := s.repo.Create(ctx, repository.CreateUserInput{Name: name, Email: email, Password: passwordHash, Active: active})
	if err != nil {
		logger.Error(ctx, "create user failed", zap.String("email", email), zap.Error(err))
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) GetUserByID(ctx context.Context, id int64) (*dto.UserResponse, error) {
	logger.Info(ctx, "query user profile", zap.Int64("user_id", id))
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "query user profile failed", zap.Int64("user_id", id), zap.Error(err))
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) ListUsers(ctx context.Context, req dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error) {
	paging := response.NormalizePagination(req.Page, req.PageSize)
	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))

	logger.Info(ctx, "list users", zap.Int("page", paging.Page), zap.Int("page_size", paging.PageSize))
	users, total, err := s.repo.ListUsers(ctx, repository.ListUsersInput{
		Offset: paging.Offset,
		Limit:  paging.Limit,
		Name:   name,
		Email:  email,
		Active: req.Active,
	})
	if err != nil {
		logger.Error(ctx, "list users failed", zap.Error(err))
		return response.PaginatedData[dto.UserResponse]{}, response.FromError(err)
	}

	items := make([]dto.UserResponse, 0, len(users))
	for _, user := range users {
		items = append(items, *toUserResponse(user))
	}
	return response.NewPaginatedData(items, response.NewPagination(paging.Page, paging.PageSize, total)), nil
}

func toUserResponse(user *ent.User) *dto.UserResponse {
	return &dto.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
