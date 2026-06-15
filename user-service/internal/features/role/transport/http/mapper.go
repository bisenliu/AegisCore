package rolehttp

import (
	"errors"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/pagination"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func toRoleResponse(role roledomain.Role) RoleResponse {
	return RoleResponse{RoleID: role.RoleID.String(), Name: role.Name, Description: role.Description, Active: role.Active, System: role.IsSystem, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}
}

func toRoleListResponse(result *rolequery.ListRolesResult) pagination.PaginatedData[RoleResponse] {
	items := toRoleResponses(result.Items)
	return pagination.NewPaginatedData(items, pagination.NewPagination(result.PageSize, result.NextCursor, result.HasNext))
}

func toRoleResponses(items []roledomain.Role) []RoleResponse {
	result := make([]RoleResponse, 0, len(items))
	for i := range items {
		result = append(result, toRoleResponse(items[i]))
	}
	return result
}

func toCommandRoleResponses(result *rolecommand.RolesResult) []RoleResponse {
	return toRoleResponses(result.Items)
}

func toPermissionResponse(permission roleapplication.PermissionReference) PermissionResponse {
	return PermissionResponse{PermissionID: permission.PermissionID.String(), Name: permission.Name, Description: permission.Description, Module: permission.Module, HTTPMethod: permission.HTTPMethod, PathTemplate: permission.PathTemplate, Active: permission.Active, System: permission.IsSystem, CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt}
}

func toPermissionResponses(items []roleapplication.PermissionReference) []PermissionResponse {
	result := make([]PermissionResponse, 0, len(items))
	for i := range items {
		result = append(result, toPermissionResponse(items[i]))
	}
	return result
}

func toRoleHTTPError(err error) error {
	switch {
	case errors.Is(err, roledomain.ErrRoleAlreadyExists):
		return contracterrors.ConflictError(messages.RoleAlreadyExists)
	case errors.Is(err, roledomain.ErrRoleNotFound):
		return contracterrors.NotFoundError(messages.RoleNotFound)
	case errors.Is(err, roledomain.ErrRoleInvalid):
		return contracterrors.ValidationFailedError(messages.InvalidRole)
	case errors.Is(err, roledomain.ErrSystemRoleProtected):
		return contracterrors.ConflictError(messages.SystemRoleProtected)
	case errors.Is(err, roledomain.ErrUserRoleAlreadyExists):
		return contracterrors.ConflictError(messages.UserRoleAlreadyExists)
	case errors.Is(err, roledomain.ErrUserRoleNotFound):
		return contracterrors.NotFoundError(messages.UserRoleNotFound)
	case errors.Is(err, roledomain.ErrRolePermissionAlreadyExists):
		return contracterrors.ConflictError(messages.RolePermissionAlreadyExists)
	case errors.Is(err, roledomain.ErrRolePermissionNotFound):
		return contracterrors.NotFoundError(messages.RolePermissionNotFound)
	case errors.Is(err, userdomain.ErrUserNotFound):
		return contracterrors.NotFoundError(messages.UserNotFound)
	case errors.Is(err, permissiondomain.ErrPermissionNotFound):
		return contracterrors.NotFoundError(messages.PermissionNotFound)
	default:
		return contracterrors.FromError(err)
	}
}
