package service

import (
	"context"
	"strings"

	"github.com/aegiscore/common/logger"
	commonpassword "github.com/aegiscore/common/password"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/apperror"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserService interface {
	CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error)
	GetUserByID(ctx context.Context, userID string) (*dto.UserResponse, error)
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
	username := strings.TrimSpace(req.Username)
	plainPassword := strings.TrimSpace(req.Password)
	if name == "" {
		return nil, response.ValidationFailedError(apperror.MsgInvalidUserName)
	}
	if username == "" {
		return nil, response.ValidationFailedError(apperror.MsgInvalidUserName)
	}
	if plainPassword == "" {
		return nil, response.ValidationFailedError(apperror.MsgInvalidPassword)
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	logger.Info(ctx, "create user", zap.String("username", username))
	exists, err := s.repo.ExistsByUsername(ctx, username)
	if err != nil {
		logger.Error(ctx, "check username failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}
	if exists {
		return nil, response.ConflictError(apperror.MsgUserAlreadyExists)
	}

	passwordHash, err := commonpassword.Hash(plainPassword)
	if err != nil {
		logger.Error(ctx, "hash user password failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}

	userID, err := uuid.NewV7()
	if err != nil {
		logger.Error(ctx, "generate user id failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}

	user, err := s.repo.Create(ctx, repository.CreateUserInput{Name: name, UserID: userID, Username: username, Password: passwordHash, Active: active})
	if err != nil {
		logger.Error(ctx, "create user failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) GetUserByID(ctx context.Context, userID string) (*dto.UserResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, response.BadRequestError(apperror.MsgInvalidUserID)
	}
	logger.Info(ctx, "query user profile", zap.String("user_id", userID))
	user, err := s.repo.GetByUserID(ctx, parsedUserID)
	if err != nil {
		logger.Error(ctx, "query user profile failed", zap.String("user_id", userID), zap.Error(err))
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) ListUsers(ctx context.Context, req dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error) {
	paging := response.NormalizePagination(req.Page, req.PageSize)
	name := strings.TrimSpace(req.Name)
	username := strings.TrimSpace(req.Username)

	logger.Info(ctx, "list users", zap.Int("page", paging.Page), zap.Int("page_size", paging.PageSize))
	users, total, err := s.repo.ListUsers(ctx, repository.ListUsersInput{
		Offset:   paging.Offset,
		Limit:    paging.Limit,
		Name:     name,
		Username: username,
		Active:   req.Active,
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
		UserID:    user.UserID.String(),
		Name:      user.Name,
		Username:  user.Username,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
