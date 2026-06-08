package repository

import (
	"context"

	"github.com/aegiscore/user-services/internal/domain"
	"github.com/google/uuid"
)

// UserProfileRepository 定义用户资料创建和查询操作。
type UserProfileRepository interface {
	Create(ctx context.Context, input CreateUserInput) (*domain.User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	ListUsers(ctx context.Context, input ListUsersInput) ([]domain.User, int, error)
}

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

// UserRepository 聚合资料、凭证和 token version 持久化契约。
type UserRepository interface {
	UserProfileRepository
	UserCredentialRepository
	UserTokenVersionRepository
}

// CreateUserInput 包含规范化后的用户创建数据和已哈希密码。
type CreateUserInput struct {
	Nickname     string
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       domain.UserStatus
}

// UpdateCredentialsInput 包含改密时使用的新凭证和目标状态。
type UpdateCredentialsInput struct {
	UserID       uuid.UUID
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
