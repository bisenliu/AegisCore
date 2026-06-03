package repository

import (
	"context"

	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, input CreateUserInput) (*ent.User, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	GetByUsername(ctx context.Context, username string) (*ent.User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*ent.User, error)
	GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	UpdateCredentials(ctx context.Context, input UpdateCredentialsInput) (int64, error)
	ListUsers(ctx context.Context, input ListUsersInput) ([]*ent.User, int, error)
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
