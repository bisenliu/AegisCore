package domain

import (
	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	reasonPermissionNotFound        contracterrors.Reason = "permission_not_found"
	reasonPermissionAlreadyExists   contracterrors.Reason = "permission_already_exists"
	reasonPermissionInvalid         contracterrors.Reason = "permission_invalid"
	reasonSystemPermissionProtected contracterrors.Reason = "system_permission_protected"
)

var (
	// ErrPermissionNotFound 表示权限目录中不存在目标权限，并可直接渲染为未找到响应。
	ErrPermissionNotFound = contracterrors.New(contracterrors.KindNotFound, reasonPermissionNotFound, contracterrors.CodeNotFound, messages.PermissionNotFound)
	// ErrPermissionAlreadyExists 表示 HTTP 方法和路径模板唯一身份冲突，并可直接渲染为冲突响应。
	ErrPermissionAlreadyExists = contracterrors.New(contracterrors.KindConflict, reasonPermissionAlreadyExists, contracterrors.CodeConflict, messages.PermissionAlreadyExists)
	// ErrPermissionInvalid 表示权限领域输入不满足规则，并可直接渲染为参数校验失败响应。
	ErrPermissionInvalid = contracterrors.New(contracterrors.KindValidation, reasonPermissionInvalid, contracterrors.CodeValidationFailed, messages.InvalidPermission)
	// ErrSystemPermissionProtected 表示系统权限的受保护字段不允许被破坏性修改，并可直接渲染为冲突响应。
	ErrSystemPermissionProtected = contracterrors.New(contracterrors.KindConflict, reasonSystemPermissionProtected, contracterrors.CodeConflict, messages.SystemPermissionProtected)
)
