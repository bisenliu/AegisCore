package query

import (
	"context"

	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
)

// UserQueryService 定义用户资料读侧用例。
type UserQueryService interface {
	GetUserByID(ctx context.Context, query GetUserByIDQuery) (*GetUserResult, error)
	ListUsers(ctx context.Context, query ListUsersQuery) (*ListUsersResult, error)
}

type userQueryService struct {
	store userapplication.UserProfileStore
}

// NewUserQueryService 根据仓储依赖构造用户资料读侧服务。
func NewUserQueryService(store userapplication.UserProfileStore) UserQueryService {
	return &userQueryService{store: store}
}
