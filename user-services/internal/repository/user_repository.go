package repository

import (
	"context"

	"github.com/aegiscore/user-services/internal/domain"
	"github.com/google/uuid"
)

// UserCredentialRepository 定义认证流程使用的凭证查询和更新操作。
type UserCredentialRepository interface {
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	UpdateCredentials(ctx context.Context, input UpdateCredentialsInput) (int64, error)
}

// UserTokenVersionRepository 定义认证失效控制使用的 token version 操作。
type UserTokenVersionRepository interface {
	GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
}

// UserRepository 聚合认证凭证和 token version 持久化契约。
type UserRepository interface {
	UserCredentialRepository
	UserTokenVersionRepository
}

// UpdateCredentialsInput 包含改密时使用的新凭证和目标状态。
type UpdateCredentialsInput struct {
	UserID       uuid.UUID
	PasswordHash string
	Status       domain.UserStatus
}
