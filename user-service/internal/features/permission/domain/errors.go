package domain

import (
	contracterrors "github.com/aegiscore/platform/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	// reasonPermissionNotFound 是权限在系统权限目录中未找到的错误原因标识。
	reasonPermissionNotFound contracterrors.Reason = "permission_not_found"
	// reasonPermissionInvalid 是权限标识符或参数不合法的错误原因标识。
	reasonPermissionInvalid contracterrors.Reason = "permission_invalid"
)

var (
	// ErrPermissionNotFound 表示权限目录中不存在目标权限，并可直接渲染为未找到响应。
	ErrPermissionNotFound = contracterrors.New(contracterrors.KindNotFound, reasonPermissionNotFound, contracterrors.CodeNotFound, messages.PermissionNotFound)
	// ErrPermissionInvalid 表示权限领域输入不满足规则，并可直接渲染为参数校验失败响应。
	ErrPermissionInvalid = contracterrors.New(contracterrors.KindValidation, reasonPermissionInvalid, contracterrors.CodeValidationFailed, messages.InvalidPermission)
)
