package domain

import "errors"

var (
	// ErrRoleNotFound 表示角色目录中不存在目标角色。
	ErrRoleNotFound = errors.New("role not found")
	// ErrRoleAlreadyExists 表示角色唯一身份冲突。
	ErrRoleAlreadyExists = errors.New("role already exists")
	// ErrRoleInvalid 表示角色领域输入不满足规则。
	ErrRoleInvalid = errors.New("role invalid")
	// ErrSystemRoleProtected 表示系统角色不允许被破坏性修改。
	ErrSystemRoleProtected = errors.New("system role protected")
	// ErrUserRoleAlreadyExists 表示用户角色绑定已存在。
	ErrUserRoleAlreadyExists = errors.New("user role already exists")
	// ErrUserRoleNotFound 表示用户角色绑定不存在。
	ErrUserRoleNotFound = errors.New("user role not found")
	// ErrRolePermissionAlreadyExists 表示角色权限绑定已存在。
	ErrRolePermissionAlreadyExists = errors.New("role permission already exists")
	// ErrRolePermissionNotFound 表示角色权限绑定不存在。
	ErrRolePermissionNotFound = errors.New("role permission not found")
)
