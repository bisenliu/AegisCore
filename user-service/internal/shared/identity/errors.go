package identity

import "errors"

var (
	// ErrUserNotFound 表示仓储查询未找到目标用户。
	ErrUserNotFound = errors.New("user not found")
	// ErrUserAlreadyExists 表示用户创建与已存在唯一 username 冲突。
	ErrUserAlreadyExists = errors.New("user already exists")
)
