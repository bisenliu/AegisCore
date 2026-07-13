package permissionhttp

import (
	"strings"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
	permissioncommand "github.com/aegiscore/user-service/internal/features/permission/application/command"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	"github.com/aegiscore/user-service/internal/messages"
)

// prepareListPermissionsQuery 将权限列表 HTTP 请求转换为应用层查询。
func prepareListPermissionsQuery(req ListPermissionsRequest) (permissionquery.ListPermissionsQuery, error) {
	cursorText := strings.TrimSpace(req.Cursor)
	var cursor *uuid.UUID
	if cursorText != "" {
		parsed, err := uuid.Parse(cursorText)
		if err != nil {
			return permissionquery.ListPermissionsQuery{}, contracterrors.BadRequestError(messages.InvalidPermission)
		}
		cursor = &parsed
	}
	pageSize := pagination.NormalizePageSize(req.PageSize)
	return permissionquery.ListPermissionsQuery{
		Cursor:     cursor,
		PageSize:   pageSize,
		Limit:      pageSize,
		Module:     strings.TrimSpace(req.Module),
		HTTPMethod: strings.TrimSpace(req.HTTPMethod),
		Active:     req.Active,
		IsSystem:   req.System,
	}, nil
}

// prepareCreatePermissionCommand 将创建权限 HTTP 请求转换为应用层命令。
func prepareCreatePermissionCommand(req CreatePermissionRequest) (permissioncommand.CreatePermissionCommand, error) {
	return permissioncommand.CreatePermissionCommand{
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		Module:       strings.TrimSpace(req.Module),
		HTTPMethod:   strings.TrimSpace(req.HTTPMethod),
		PathTemplate: strings.TrimSpace(req.PathTemplate),
		Active:       req.Active,
	}, nil
}

// prepareGetPermissionQuery 将权限 ID URI 请求转换为应用层查询。
func prepareGetPermissionQuery(req PermissionIDRequest) (permissionquery.GetPermissionQuery, error) {
	permissionID, err := parsePermissionID(req.PermissionID)
	if err != nil {
		return permissionquery.GetPermissionQuery{}, err
	}
	return permissionquery.GetPermissionQuery{PermissionID: permissionID}, nil
}

// prepareUpdatePermissionCommand 将权限更新 HTTP 请求转换为应用层命令。
func prepareUpdatePermissionCommand(req UpdatePermissionHTTPRequest) (permissioncommand.UpdatePermissionCommand, error) {
	permissionID, err := parsePermissionID(req.PermissionID)
	if err != nil {
		return permissioncommand.UpdatePermissionCommand{}, err
	}
	return permissioncommand.UpdatePermissionCommand{
		PermissionID: permissionID,
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		Module:       strings.TrimSpace(req.Module),
		HTTPMethod:   strings.TrimSpace(req.HTTPMethod),
		PathTemplate: strings.TrimSpace(req.PathTemplate),
		Active:       req.Active,
	}, nil
}

// prepareSetPermissionActiveCommand 将权限 ID URI 请求转换为启停命令。
func prepareSetPermissionActiveCommand(req PermissionIDRequest) (permissioncommand.SetPermissionActiveCommand, error) {
	permissionID, err := parsePermissionID(req.PermissionID)
	if err != nil {
		return permissioncommand.SetPermissionActiveCommand{}, err
	}
	return permissioncommand.SetPermissionActiveCommand{PermissionID: permissionID}, nil
}

// prepareUserEffectivePermissionsQuery 将用户 ID URI 请求转换为有效权限查询。
func prepareUserEffectivePermissionsQuery(req UserIDRequest) (permissionquery.UserEffectivePermissionsQuery, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return permissionquery.UserEffectivePermissionsQuery{}, contracterrors.BadRequestError(messages.InvalidUserID)
	}
	return permissionquery.UserEffectivePermissionsQuery{UserID: userID}, nil
}

func parsePermissionID(raw string) (uuid.UUID, error) {
	permissionID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidPermission)
	}
	return permissionID, nil
}
