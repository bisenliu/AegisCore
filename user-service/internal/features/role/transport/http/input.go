package rolehttp

import (
	"strings"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	"github.com/aegiscore/user-service/internal/messages"
)

// prepareListRolesQuery 将角色列表 HTTP 请求转换为应用层查询。
func prepareListRolesQuery(req ListRolesRequest) (rolequery.ListRolesQuery, error) {
	cursorText := strings.TrimSpace(req.Cursor)
	var cursor *uuid.UUID
	if cursorText != "" {
		parsed, err := uuid.Parse(cursorText)
		if err != nil {
			return rolequery.ListRolesQuery{}, contracterrors.BadRequestError(messages.InvalidRole)
		}
		cursor = &parsed
	}
	pageSize := pagination.NormalizePageSize(req.PageSize)
	return rolequery.ListRolesQuery{Cursor: cursor, PageSize: pageSize, Limit: pageSize, Active: req.Active, IsSystem: req.System}, nil
}

// prepareCreateRoleCommand 将创建角色 HTTP 请求转换为应用层命令。
func prepareCreateRoleCommand(req CreateRoleRequest) (rolecommand.CreateRoleCommand, error) {
	return rolecommand.CreateRoleCommand{Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Active: req.Active}, nil
}

// prepareUpdateRoleCommand 将角色更新 HTTP 请求转换为应用层命令。
func prepareUpdateRoleCommand(req UpdateRoleHTTPRequest) (rolecommand.UpdateRoleCommand, error) {
	roleID, err := parseRoleID(req.RoleID)
	if err != nil {
		return rolecommand.UpdateRoleCommand{}, err
	}
	return rolecommand.UpdateRoleCommand{RoleID: roleID, Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Active: req.Active}, nil
}

// prepareSetRoleActiveCommand 将角色状态 HTTP 请求转换为应用层命令。
func prepareSetRoleActiveCommand(req SetRoleStatusHTTPRequest) (rolecommand.SetRoleActiveCommand, error) {
	roleID, err := parseRoleID(req.RoleID)
	if err != nil {
		return rolecommand.SetRoleActiveCommand{}, err
	}
	return rolecommand.SetRoleActiveCommand{RoleID: roleID, Active: req.Active}, nil
}

// prepareGetRoleQuery 将角色 ID URI 请求转换为应用层查询。
func prepareGetRoleQuery(req RoleIDRequest) (rolequery.GetRoleQuery, error) {
	roleID, err := parseRoleID(req.RoleID)
	if err != nil {
		return rolequery.GetRoleQuery{}, err
	}
	return rolequery.GetRoleQuery{RoleID: roleID}, nil
}

// prepareUserRolesQuery 将用户 ID URI 请求转换为用户角色查询。
func prepareUserRolesQuery(req UserIDRequest) (rolequery.UserRolesQuery, error) {
	userID, err := parseUserID(req.UserID)
	if err != nil {
		return rolequery.UserRolesQuery{}, err
	}
	return rolequery.UserRolesQuery{UserID: userID}, nil
}

// prepareReplaceUserRolesCommand 将替换用户角色 HTTP 请求转换为应用层命令。
func prepareReplaceUserRolesCommand(req ReplaceUserRolesHTTPRequest) (rolecommand.ReplaceUserRolesCommand, error) {
	userID, err := parseUserID(req.UserID)
	if err != nil {
		return rolecommand.ReplaceUserRolesCommand{}, err
	}
	roleIDs, err := parseUUIDs(req.RoleIDs, messages.InvalidRole)
	if err != nil {
		return rolecommand.ReplaceUserRolesCommand{}, err
	}
	return rolecommand.ReplaceUserRolesCommand{UserID: userID, RoleIDs: roleIDs}, nil
}

// prepareUserRoleCommand 将单个用户角色绑定 HTTP 请求转换为应用层命令。
func prepareUserRoleCommand(req UserRoleHTTPRequest) (rolecommand.UserRoleCommand, error) {
	userID, err := parseUserID(req.UserID)
	if err != nil {
		return rolecommand.UserRoleCommand{}, err
	}
	roleID, err := parseRoleID(req.RoleID)
	if err != nil {
		return rolecommand.UserRoleCommand{}, err
	}
	return rolecommand.UserRoleCommand{UserID: userID, RoleID: roleID}, nil
}

// prepareRolePermissionsQuery 将角色 ID URI 请求转换为角色权限查询。
func prepareRolePermissionsQuery(req RoleIDRequest) (rolequery.RolePermissionsQuery, error) {
	roleID, err := parseRoleID(req.RoleID)
	if err != nil {
		return rolequery.RolePermissionsQuery{}, err
	}
	return rolequery.RolePermissionsQuery{RoleID: roleID}, nil
}

// prepareReplaceRolePermissionsCommand 将替换角色权限 HTTP 请求转换为应用层命令。
func prepareReplaceRolePermissionsCommand(req ReplaceRolePermissionsHTTPRequest) (rolecommand.ReplaceRolePermissionsCommand, error) {
	roleID, err := parseRoleID(req.RoleID)
	if err != nil {
		return rolecommand.ReplaceRolePermissionsCommand{}, err
	}
	permissionIDs, err := parseUUIDs(req.PermissionIDs, messages.InvalidPermission)
	if err != nil {
		return rolecommand.ReplaceRolePermissionsCommand{}, err
	}
	return rolecommand.ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: permissionIDs}, nil
}

// prepareRolePermissionCommand 将单个角色权限绑定 HTTP 请求转换为应用层命令。
func prepareRolePermissionCommand(req RolePermissionHTTPRequest) (rolecommand.RolePermissionCommand, error) {
	roleID, err := parseRoleID(req.RoleID)
	if err != nil {
		return rolecommand.RolePermissionCommand{}, err
	}
	permissionID, err := parsePermissionID(req.PermissionID)
	if err != nil {
		return rolecommand.RolePermissionCommand{}, err
	}
	return rolecommand.RolePermissionCommand{RoleID: roleID, PermissionID: permissionID}, nil
}

func parseUserID(raw string) (uuid.UUID, error) {
	userID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidUserID)
	}
	return userID, nil
}

func parseRoleID(raw string) (uuid.UUID, error) {
	roleID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidRole)
	}
	return roleID, nil
}

func parsePermissionID(raw string) (uuid.UUID, error) {
	permissionID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, contracterrors.BadRequestError(messages.InvalidPermission)
	}
	return permissionID, nil
}

func parseUUIDs(rawValues []string, invalidMessage string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(rawValues))
	for _, raw := range rawValues {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, contracterrors.BadRequestError(invalidMessage)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
