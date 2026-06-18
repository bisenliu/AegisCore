package id

import "github.com/google/uuid"

// NewUUID 生成当前跨服务默认 UUID。
func NewUUID() (uuid.UUID, error) {
	return uuid.NewV7()
}

// NewUUIDString 生成当前跨服务默认 UUID 字符串。
func NewUUIDString() (string, error) {
	value, err := NewUUID()
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

// MustNewUUID 生成当前跨服务默认 UUID，生成失败时 panic。
func MustNewUUID() uuid.UUID {
	value, err := NewUUID()
	if err != nil {
		panic(err)
	}
	return value
}

// MustNewUUIDString 生成当前跨服务默认 UUID 字符串，生成失败时 panic。
func MustNewUUIDString() string {
	return MustNewUUID().String()
}
