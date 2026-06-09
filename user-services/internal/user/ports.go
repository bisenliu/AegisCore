package user

import (
	"context"

	"github.com/aegiscore/common/contract/response"
	"github.com/google/uuid"
)

// UserService 定义暴露给 HTTP controller 的用户资料用例。
type UserService interface {
	CreateUser(ctx context.Context, cmd CreateUserCommand) (*UserResponse, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*UserResponse, error)
	ListUsers(ctx context.Context, query ListUsersQuery) (response.PaginatedData[UserResponse], error)
}

// UserProfileStore 定义用户资料 service 实际消费的持久化端口。
type UserProfileStore interface {
	Create(ctx context.Context, input CreateUserInput) (*User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*User, error)
	ListUsers(ctx context.Context, input ListUsersInput) ([]User, int, error)
}

// CreateUserInput 包含规范化后的用户创建数据和已哈希密码。
type CreateUserInput struct {
	Nickname     string
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       UserStatus
}

// ListUsersInput 包含用户列表查询使用的规范化分页和过滤条件。
type ListUsersInput struct {
	Offset   int
	Limit    int
	Nickname string
	Username string
	Status   *UserStatus
}
