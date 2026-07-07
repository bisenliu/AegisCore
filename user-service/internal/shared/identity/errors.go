package identity

import (
	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	reasonUserNotFound      contracterrors.Reason = "user_not_found"
	reasonUserAlreadyExists contracterrors.Reason = "user_already_exists"
)

var (
	// ErrUserNotFound 表示仓储查询未找到目标用户，并可直接渲染为未找到响应。
	ErrUserNotFound = contracterrors.New(contracterrors.KindNotFound, reasonUserNotFound, contracterrors.CodeNotFound, messages.UserNotFound)
	// ErrUserAlreadyExists 表示用户创建与已存在唯一 username 冲突，并可直接渲染为冲突响应。
	ErrUserAlreadyExists = contracterrors.New(contracterrors.KindConflict, reasonUserAlreadyExists, contracterrors.CodeConflict, messages.UserAlreadyExists)
)
