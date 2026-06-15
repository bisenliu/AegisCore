package identity

import (
	"encoding/json"
	"strconv"
)

// UserStatus 是用户持久化生命周期和认证状态。
type UserStatus int64

const (
	// UserStatusNormal 表示用户状态正常，可正常登录并访问受保护资源。
	UserStatusNormal UserStatus = 100
	// UserStatusDisabled 表示用户已被冻结或停用，不允许登录。
	UserStatusDisabled UserStatus = 200
	// UserStatusMustChangePassword 表示用户必须先完成密码修改，只允许获取改密令牌。
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

// CanLogin 返回 s 是否允许常规登录和 access token 使用。
func (s UserStatus) CanLogin() bool {
	return s == UserStatusNormal
}

// RequiresPasswordChange 返回 s 是否要求先完成强制改密流程。
func (s UserStatus) RequiresPasswordChange() bool {
	return s == UserStatusMustChangePassword
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
