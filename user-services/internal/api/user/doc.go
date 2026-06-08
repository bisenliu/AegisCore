package userapi

import "github.com/aegiscore/common/contract/response"

// UserListResponseDoc 描述分页用户列表 Swagger 响应载荷。
type UserListResponseDoc struct {
	Items      []UserResponse      `json:"items"`
	Pagination response.Pagination `json:"pagination"`
}
