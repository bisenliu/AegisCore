package service

import (
	"context"
	"errors"
	"strings"

	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/password"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/errmsg"
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
	nickname := strings.TrimSpace(req.Nickname)
	username := strings.TrimSpace(req.Username)
	plainPassword := strings.TrimSpace(req.Password)
	if nickname == "" {
		return nil, response.ValidationFailedError(errmsg.MsgInvalidUserName)
	}
	if username == "" {
		return nil, response.ValidationFailedError(errmsg.MsgInvalidUserName)
	}
	if plainPassword == "" {
		return nil, response.ValidationFailedError(errmsg.MsgInvalidPassword)
	}
	status := domain.UserStatusNormal
	if req.Status != nil {
		status = *req.Status
	}

	logger.Info(ctx, "create user", zap.String("username", username))
	exists, err := s.repo.ExistsByUsername(ctx, username)
	if err != nil {
		logger.Error(ctx, "check username failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}
	if exists {
		return nil, response.ConflictError(errmsg.MsgUserAlreadyExists)
	}

	passwordHash, err := password.Hash(plainPassword)
	if err != nil {
		logger.Error(ctx, "hash user password failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}

	userID, err := uuid.NewV7()
	if err != nil {
		logger.Error(ctx, "generate user id failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}

	user, err := s.repo.Create(ctx, repository.CreateUserInput{Nickname: nickname, UserID: userID, Username: username, PasswordHash: passwordHash, Status: status})
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			return nil, response.ConflictError(errmsg.MsgUserAlreadyExists)
		}
		logger.Error(ctx, "create user failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) GetUserByID(ctx context.Context, userID string) (*dto.UserResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, response.BadRequestError(errmsg.MsgInvalidUserID)
	}
	logger.Info(ctx, "query user profile", zap.String("user_id", userID))
	user, err := s.repo.GetByUserID(ctx, parsedUserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "query user profile failed", zap.String("user_id", userID), zap.Error(err))
		return nil, response.FromError(err)
	}
	return toUserResponse(user), nil
}

func (s *userService) ListUsers(ctx context.Context, req dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error) {
	paging := response.NormalizePagination(req.Page, req.PageSize)
	nickname := strings.TrimSpace(req.Nickname)
	username := strings.TrimSpace(req.Username)

	logger.Info(ctx, "list users", zap.Int("page", paging.Page), zap.Int("page_size", paging.PageSize))
	users, total, err := s.repo.ListUsers(ctx, repository.ListUsersInput{
		Offset:   paging.Offset,
		Limit:    paging.Limit,
		Nickname: nickname,
		Username: username,
		Status:   req.Status,
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
		Nickname:  user.Nickname,
		Username:  user.Username,
		Status:    domain.UserStatus(user.Status),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
