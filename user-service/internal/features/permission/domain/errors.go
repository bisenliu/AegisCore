package domain

import (
	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	reasonPermissionNotFound contracterrors.Reason = "permission_not_found"
	reasonPermissionInvalid  contracterrors.Reason = "permission_invalid"
)

var (
	// ErrPermissionNotFound 表示权限目录中不存在目标权限，并可直接渲染为未找到响应。
	ErrPermissionNotFound = contracterrors.New(contracterrors.KindNotFound, reasonPermissionNotFound, contracterrors.CodeNotFound, messages.PermissionNotFound)
	// ErrPermissionInvalid 表示权限领域输入不满足规则，并可直接渲染为参数校验失败响应。
	ErrPermissionInvalid = contracterrors.New(contracterrors.KindValidation, reasonPermissionInvalid, contracterrors.CodeValidationFailed, messages.InvalidPermission)
)
