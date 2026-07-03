package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleQueryServiceListRolesNormalizesLimitAndNextCursor(t *testing.T) {
	firstRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")
	lastRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000102")
	roles := NewMockRoleStore(gomock.NewController(t))
	var listInput roleapplication.ListRolesInput
	roles.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input roleapplication.ListRolesInput) ([]roledomain.Role, bool, error) {
		listInput = input
		return []roledomain.Role{{RoleID: firstRoleID, Name: "operator", Active: true}, {RoleID: lastRoleID, Name: "auditor", Active: true}}, true, nil
	})
	service := NewRoleQueryService(roles, NewMockUserRoleStore(gomock.NewController(t)), NewMockRolePermissionStore(gomock.NewController(t)))

	result, err := service.ListRoles(context.Background(), ListRolesQuery{})
	require.NoError(t, err)
	require.Greater(t, listInput.Limit, 0)
	require.Equal(t, listInput.Limit, result.PageSize)
	require.True(t, result.HasNext)
	require.Equal(t, lastRoleID.String(), result.NextCursor)
}

func TestRoleQueryServiceGetAndBindingQueriesPassThrough(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000103")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000104")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000105")
	roles := NewMockRoleStore(gomock.NewController(t))
	roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&roledomain.Role{RoleID: roleID, Name: "operator", Active: true}, nil)
	userRoles := NewMockUserRoleStore(gomock.NewController(t))
	userRoles.EXPECT().ListByUserID(gomock.Any(), userID).Return([]roledomain.Role{{RoleID: roleID, Name: "operator", Active: true}}, nil)
	rolePermissions := NewMockRolePermissionStore(gomock.NewController(t))
	rolePermissions.EXPECT().ListByRoleID(gomock.Any(), roleID).Return([]roleapplication.PermissionReference{{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}}, nil)
	service := NewRoleQueryService(roles, userRoles, rolePermissions)

	roleResult, err := service.GetRole(context.Background(), GetRoleQuery{RoleID: roleID})
	require.NoError(t, err)
	require.Equal(t, roleID, roleResult.Role.RoleID)
	require.Equal(t, "operator", roleResult.Role.Name)

	rolesResult, err := service.ListUserRoles(context.Background(), UserRolesQuery{UserID: userID})
	require.NoError(t, err)
	require.Len(t, rolesResult.Items, 1)
	require.Equal(t, roleID, rolesResult.Items[0].RoleID)

	permissionsResult, err := service.ListRolePermissions(context.Background(), RolePermissionsQuery{RoleID: roleID})
	require.NoError(t, err)
	require.Len(t, permissionsResult.Items, 1)
	require.Equal(t, permissionID, permissionsResult.Items[0].PermissionID)
}
