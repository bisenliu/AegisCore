package domain

import "errors"

var (
	// ErrPermissionNotFound 表示权限目录中不存在目标权限。
	ErrPermissionNotFound = errors.New("permission not found")
	// ErrPermissionAlreadyExists 表示 HTTP 方法和路径模板唯一身份冲突。
	ErrPermissionAlreadyExists = errors.New("permission already exists")
	// ErrPermissionInvalid 表示权限领域输入不满足规则。
	ErrPermissionInvalid = errors.New("permission invalid")
	// ErrSystemPermissionProtected 表示系统权限的受保护字段不允许被破坏性修改。
	ErrSystemPermissionProtected = errors.New("system permission protected")
)
