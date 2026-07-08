package domain

import (
	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	reasonRoleNotFound                contracterrors.Reason = "role_not_found"
	reasonRoleAlreadyExists           contracterrors.Reason = "role_already_exists"
	reasonRoleInvalid                 contracterrors.Reason = "role_invalid"
	reasonSystemRoleProtected         contracterrors.Reason = "system_role_protected"
	reasonRoleInactive                contracterrors.Reason = "role_inactive"
	reasonUserRoleAlreadyExists       contracterrors.Reason = "user_role_already_exists"
	reasonUserRoleNotFound            contracterrors.Reason = "user_role_not_found"
	reasonRolePermissionAlreadyExists contracterrors.Reason = "role_permission_already_exists"
	reasonRolePermissionNotFound      contracterrors.Reason = "role_permission_not_found"
)

var (
	// ErrRoleNotFound 表示角色目录中不存在目标角色，并可直接渲染为未找到响应。
	ErrRoleNotFound = contracterrors.New(contracterrors.KindNotFound, reasonRoleNotFound, contracterrors.CodeNotFound, messages.RoleNotFound)
	// ErrRoleAlreadyExists 表示角色唯一身份冲突，并可直接渲染为冲突响应。
	ErrRoleAlreadyExists = contracterrors.New(contracterrors.KindConflict, reasonRoleAlreadyExists, contracterrors.CodeConflict, messages.RoleAlreadyExists)
	// ErrRoleInvalid 表示角色领域输入不满足规则，并可直接渲染为参数校验失败响应。
	ErrRoleInvalid = contracterrors.New(contracterrors.KindValidation, reasonRoleInvalid, contracterrors.CodeValidationFailed, messages.InvalidRole)
	// ErrSystemRoleProtected 表示系统角色不允许被破坏性修改，并可直接渲染为冲突响应。
	ErrSystemRoleProtected = contracterrors.New(contracterrors.KindConflict, reasonSystemRoleProtected, contracterrors.CodeConflict, messages.SystemRoleProtected)
	// ErrRoleInactive 表示目标角色已停用，不能用于新的用户角色绑定，并可直接渲染为冲突响应。
	ErrRoleInactive = contracterrors.New(contracterrors.KindConflict, reasonRoleInactive, contracterrors.CodeConflict, messages.RoleInactive)
	// ErrUserRoleAlreadyExists 表示用户角色绑定已存在，并可直接渲染为冲突响应。
	ErrUserRoleAlreadyExists = contracterrors.New(contracterrors.KindConflict, reasonUserRoleAlreadyExists, contracterrors.CodeConflict, messages.UserRoleAlreadyExists)
	// ErrUserRoleNotFound 表示用户角色绑定不存在，并可直接渲染为未找到响应。
	ErrUserRoleNotFound = contracterrors.New(contracterrors.KindNotFound, reasonUserRoleNotFound, contracterrors.CodeNotFound, messages.UserRoleNotFound)
	// ErrRolePermissionAlreadyExists 表示角色权限绑定已存在，并可直接渲染为冲突响应。
	ErrRolePermissionAlreadyExists = contracterrors.New(contracterrors.KindConflict, reasonRolePermissionAlreadyExists, contracterrors.CodeConflict, messages.RolePermissionAlreadyExists)
	// ErrRolePermissionNotFound 表示角色权限绑定不存在，并可直接渲染为未找到响应。
	ErrRolePermissionNotFound = contracterrors.New(contracterrors.KindNotFound, reasonRolePermissionNotFound, contracterrors.CodeNotFound, messages.RolePermissionNotFound)
)
