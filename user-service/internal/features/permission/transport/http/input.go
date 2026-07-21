package permissionhttp

import (
	"strings"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
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
	}, nil
}

// prepareUserEffectivePermissionsQuery 将用户 ID URI 请求转换为有效权限查询。
func prepareUserEffectivePermissionsQuery(req UserIDRequest) (permissionquery.UserEffectivePermissionsQuery, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return permissionquery.UserEffectivePermissionsQuery{}, contracterrors.BadRequestError(messages.InvalidUserID)
	}
	return permissionquery.UserEffectivePermissionsQuery{UserID: userID}, nil
}
