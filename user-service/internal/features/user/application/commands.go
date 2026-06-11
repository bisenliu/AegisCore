package application

import (
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/google/uuid"
)

// CreateUserCommand 包含创建用户所需的应用层输入。
type CreateUserCommand struct {
	Nickname string
	Username string
	Password string
	Status   *userdomain.UserStatus
}

// ListUsersQuery 包含用户列表查询使用的规范化分页和过滤条件。
type ListUsersQuery struct {
	Cursor   *uuid.UUID
	PageSize int
	Limit    int
	Nickname string
	Username string
	Status   *userdomain.UserStatus
}
