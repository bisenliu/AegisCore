package permissionhttp

import (
	"strings"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	"github.com/aegiscore/user-service/internal/messages"
)

// prepareListPermissionsQuery 将权限列表 HTTP 请求转换为应用层查询。
func prepareListPermissionsQuery(req ListPermissionsRequest) (permissionquery.ListPermissionsQuery, error) {
	return permissionquery.ListPermissionsQuery{
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
