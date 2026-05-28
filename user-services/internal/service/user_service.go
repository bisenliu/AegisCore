package service

import (
	"context"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
)

type UserService interface {
	GetUserByID(ctx context.Context, id int64) (*dto.UserResponse, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) GetUserByID(ctx context.Context, id int64) (*dto.UserResponse, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, response.FromError(err)
	}
	return &dto.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}
