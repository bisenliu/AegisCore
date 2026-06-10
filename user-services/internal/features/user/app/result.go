package app

import userdomain "github.com/aegiscore/user-services/internal/features/user/domain"

// UserResult 是用户资料 use case 的 transport-neutral 输出。
type UserResult struct {
	User userdomain.User
}

// ListUsersResult 是用户列表 use case 的 transport-neutral 分页输出。
type ListUsersResult struct {
	Items    []userdomain.User
	Page     int
	PageSize int
	Total    int
}
