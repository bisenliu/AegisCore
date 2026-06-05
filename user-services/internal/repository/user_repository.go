package repository

import (
	"context"

	"github.com/aegiscore/user-services/internal/domain"
	"github.com/google/uuid"
)

type UserProfileRepository interface {
	Create(ctx context.Context, input CreateUserInput) (*domain.User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	ListUsers(ctx context.Context, input ListUsersInput) ([]domain.User, int, error)
}

type UserCredentialRepository interface {
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	UpdateCredentials(ctx context.Context, input UpdateCredentialsInput) (int64, error)
}

type UserTokenVersionRepository interface {
	GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
}

type UserRepository interface {
	UserProfileRepository
	UserCredentialRepository
	UserTokenVersionRepository
}

type CreateUserInput struct {
	Nickname     string
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       domain.UserStatus
}

type UpdateCredentialsInput struct {
	UserID       uuid.UUID
	PasswordHash string
	Status       domain.UserStatus
}

type ListUsersInput struct {
	Offset   int
	Limit    int
	Nickname string
	Username string
	Status   *domain.UserStatus
}
