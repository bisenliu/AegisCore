package domain

import (
	"encoding/json"
	"strconv"
)

type UserStatus int64

const (
	UserStatusNormal             UserStatus = 100
	UserStatusDisabled           UserStatus = 200
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
