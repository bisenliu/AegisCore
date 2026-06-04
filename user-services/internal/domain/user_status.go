package domain

import (
	"encoding/json"
	"strconv"
)

type UserStatus int64

const (
	// UserStatusNormal 表示用户状态正常，可正常登录并访问受保护资源。
	UserStatusNormal UserStatus = 100
	// UserStatusDisabled 表示用户已被冻结或停用，不允许登录。
	UserStatusDisabled UserStatus = 200
	// UserStatusMustChangePassword 表示用户必须先完成密码修改，只允许获取改密令牌。
	UserStatusMustChangePassword UserStatus = 300
)

func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusNormal, UserStatusDisabled, UserStatusMustChangePassword:
		return true
	default:
		return false
	}
}

func (s UserStatus) AllowedValues() []string {
	return []string{
		strconv.FormatInt(int64(UserStatusNormal), 10),
		strconv.FormatInt(int64(UserStatusDisabled), 10),
		strconv.FormatInt(int64(UserStatusMustChangePassword), 10),
	}
}

func (s UserStatus) CanLogin() bool {
	return s == UserStatusNormal
}

func (s *UserStatus) UnmarshalText(text []byte) error {
	value, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}
	*s = UserStatus(value)
	return nil
}

func (s *UserStatus) UnmarshalJSON(data []byte) error {
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = UserStatus(value)
	return nil
}
