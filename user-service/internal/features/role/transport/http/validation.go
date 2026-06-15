package rolehttp

import (
	"strings"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
	"github.com/aegiscore/user-service/internal/messages"
)

// NormalizeListRoles 应用分页默认值并裁剪角色列表过滤条件。
func NormalizeListRoles(req *ListRolesRequest) {
	req.Cursor = strings.TrimSpace(req.Cursor)
	req.PageSize = pagination.NormalizePageSize(req.PageSize)
	req.Limit = req.PageSize
}

// NormalizeCreateRole 在 service 处理前裁剪角色创建输入。
func NormalizeCreateRole(req *CreateRoleRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
}

// NormalizeUpdateRole 在 service 处理前裁剪角色更新输入。
func NormalizeUpdateRole(req *UpdateRoleRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
}

// ParseListCursor 将列表 cursor 转换为 UUID；空 cursor 表示第一页。
func ParseListCursor(req ListRolesRequest) (*uuid.UUID, error) {
	if req.Cursor == "" {
		return nil, nil
	}
	cursor, err := uuid.Parse(req.Cursor)
	if err != nil {
		return nil, contracterrors.BadRequestError(messages.InvalidRole)
	}
	return &cursor, nil
}

// ParseRoleID 将 URI role ID 转换为 UUID。
func ParseRoleID(req RoleIDRequest) (uuid.UUID, error) {
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidRole)
	}
	return roleID, nil
}

// ParseUserID 将 URI user ID 转换为 UUID。
func ParseUserID(req UserIDRequest) (uuid.UUID, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidUserID)
	}
	return userID, nil
}

// ParseUserRoleIDs 将 URI user ID 和 role ID 转换为 UUID。
func ParseUserRoleIDs(req UserRoleRequest) (uuid.UUID, uuid.UUID, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return uuid.Nil, uuid.Nil, contracterrors.BadRequestError(messages.InvalidUserID)
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		return uuid.Nil, uuid.Nil, contracterrors.BadRequestError(messages.InvalidRole)
	}
	return userID, roleID, nil
}

// ParseRolePermissionIDs 将 URI role ID 和 permission ID 转换为 UUID。
func ParseRolePermissionIDs(req RolePermissionRequest) (uuid.UUID, uuid.UUID, error) {
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		return uuid.Nil, uuid.Nil, contracterrors.BadRequestError(messages.InvalidRole)
	}
	permissionID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, contracterrors.BadRequestError(messages.InvalidPermission)
	}
	return roleID, permissionID, nil
}

// ParseRoleIDs 将请求体 role IDs 转换为 UUID 列表。
func ParseRoleIDs(req RoleIDsRequest) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(req.RoleIDs))
	for _, raw := range req.RoleIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, contracterrors.BadRequestError(messages.InvalidRole)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ParseRoleIDBody 将请求体 role ID 转换为 UUID。
func ParseRoleIDBody(req RoleIDBodyRequest) (uuid.UUID, error) {
	roleID, err := uuid.Parse(strings.TrimSpace(req.RoleID))
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidRole)
	}
	return roleID, nil
}

// ParsePermissionIDs 将请求体 permission IDs 转换为 UUID 列表。
func ParsePermissionIDs(req PermissionIDsRequest) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(req.PermissionIDs))
	for _, raw := range req.PermissionIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, contracterrors.BadRequestError(messages.InvalidPermission)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ParsePermissionIDBody 将请求体 permission ID 转换为 UUID。
func ParsePermissionIDBody(req PermissionIDBodyRequest) (uuid.UUID, error) {
	permissionID, err := uuid.Parse(strings.TrimSpace(req.PermissionID))
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidPermission)
	}
	return permissionID, nil
}
