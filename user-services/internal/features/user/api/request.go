package userapi

import (
	"encoding/json"
	"strconv"
)

// UserStatus 是 HTTP request DTO 使用的用户状态枚举。
type UserStatus int64

const (
	// UserStatusNormal 表示用户状态正常。
	UserStatusNormal UserStatus = 100
	// UserStatusDisabled 表示用户已被冻结或停用。
	UserStatusDisabled UserStatus = 200
	// UserStatusMustChangePassword 表示用户必须先完成密码修改。
	UserStatusMustChangePassword UserStatus = 300
)

// IsValid 返回 s 是否为已知用户状态值之一。
func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusNormal, UserStatusDisabled, UserStatusMustChangePassword:
		return true
	default:
		return false
	}
}

// AllowedValues 返回用于枚举校验消息的有效用户状态字符串值。
func (s UserStatus) AllowedValues() []string {
	return []string{
		strconv.FormatInt(int64(UserStatusNormal), 10),
		strconv.FormatInt(int64(UserStatusDisabled), 10),
		strconv.FormatInt(int64(UserStatusMustChangePassword), 10),
	}
}

// UnmarshalText 将 query 或 form 文本解析为用户状态值。
func (s *UserStatus) UnmarshalText(text []byte) error {
	value, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}
	*s = UserStatus(value)
	return nil
}

// UnmarshalJSON 解析 JSON 数字用户状态值。
func (s *UserStatus) UnmarshalJSON(data []byte) error {
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = UserStatus(value)
	return nil
}

// GetUserRequest 是通过外部 UUID 查询用户的 URI 绑定请求。
type GetUserRequest struct {
	UserID string `uri:"user_id" validate:"required,uuid" label:"用户ID" example:"018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
}

// ListUsersRequest 是分页用户列表和过滤条件的 query 绑定请求。
type ListUsersRequest struct {
	Page     int         `query:"page" label:"页码" example:"1"`
	PageSize int         `query:"page_size" label:"每页数量" example:"20"`
	Offset   int         `query:"-"`
	Limit    int         `query:"-"`
	Nickname string      `query:"nickname" label:"用户昵称" example:"Alice"`
	Username string      `query:"username" label:"用户名" example:"alice"`
	Status   *UserStatus `query:"status" validate:"omitempty,enum" label:"用户状态" example:"100"`
}

// CreateUserRequest 是创建用户资料和凭证的 JSON 请求体。
type CreateUserRequest struct {
	Nickname string      `json:"nickname" validate:"required,min=1,max=128" label:"用户昵称" example:"Alice"`
	Username string      `json:"username" validate:"required,min=1,max=255" label:"用户名" example:"alice"`
	Password string      `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
	Status   *UserStatus `json:"status,omitempty" validate:"omitempty,enum" label:"用户状态" example:"100"`
}

// SetDefaults 在校验前将缺省用户状态设置为 UserStatusNormal。
func (r *CreateUserRequest) SetDefaults() {
	if r.Status == nil {
		status := UserStatusNormal
		r.Status = &status
	}
}
