package permissionhttp

import (
	"strings"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
	"github.com/aegiscore/user-service/internal/messages"
)

// NormalizeListPermissions 应用分页默认值并裁剪权限列表过滤条件。
func NormalizeListPermissions(req *ListPermissionsRequest) {
	req.Cursor = strings.TrimSpace(req.Cursor)
	req.PageSize = pagination.NormalizePageSize(req.PageSize)
	req.Limit = req.PageSize
	req.Module = strings.TrimSpace(req.Module)
	req.HTTPMethod = strings.TrimSpace(req.HTTPMethod)
}

// NormalizeCreatePermission 在 service 处理前裁剪权限创建输入。
func NormalizeCreatePermission(req *CreatePermissionRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Module = strings.TrimSpace(req.Module)
	req.HTTPMethod = strings.TrimSpace(req.HTTPMethod)
	req.PathTemplate = strings.TrimSpace(req.PathTemplate)
}

// NormalizeUpdatePermission 在 service 处理前裁剪权限更新输入。
func NormalizeUpdatePermission(req *UpdatePermissionRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Module = strings.TrimSpace(req.Module)
	req.HTTPMethod = strings.TrimSpace(req.HTTPMethod)
	req.PathTemplate = strings.TrimSpace(req.PathTemplate)
}

// ParseListCursor 将列表 cursor 转换为 UUID；空 cursor 表示第一页。
func ParseListCursor(req ListPermissionsRequest) (*uuid.UUID, error) {
	if req.Cursor == "" {
		return nil, nil
	}
	cursor, err := uuid.Parse(req.Cursor)
	if err != nil {
		return nil, contracterrors.BadRequestError(messages.InvalidPermission)
	}
	return &cursor, nil
}

// ParsePermissionID 将 URI permission ID 转换为 UUID。
func ParsePermissionID(req PermissionIDRequest) (uuid.UUID, error) {
	permissionID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidPermission)
	}
	return permissionID, nil
}

// ParseUserID 将 URI user ID 转换为 UUID。
func ParseUserID(req UserIDRequest) (uuid.UUID, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidUserID)
	}
	return userID, nil
}
