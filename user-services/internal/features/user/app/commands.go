package app

import userdomain "github.com/aegiscore/user-services/internal/features/user/domain"

// CreateUserCommand 包含创建用户所需的应用层输入。
type CreateUserCommand struct {
	Nickname string
	Username string
	Password string
	Status   *userdomain.UserStatus
}

// ListUsersQuery 包含用户列表查询使用的规范化分页和过滤条件。
type ListUsersQuery struct {
	Page     int
	PageSize int
	Offset   int
	Limit    int
	Nickname string
	Username string
	Status   *userdomain.UserStatus
}
