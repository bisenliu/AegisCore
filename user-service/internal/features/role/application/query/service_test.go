package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
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
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if listInput.Limit <= 0 || result.PageSize != listInput.Limit {
		t.Fatalf("page size = %d, store limit = %d", result.PageSize, listInput.Limit)
	}
	if !result.HasNext || result.NextCursor != lastRoleID.String() {
		t.Fatalf("pagination result = %#v", result)
	}
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
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}
	if roleResult.Role.RoleID != roleID || roleResult.Role.Name != "operator" {
		t.Fatalf("role result = %#v", roleResult.Role)
	}

	rolesResult, err := service.ListUserRoles(context.Background(), UserRolesQuery{UserID: userID})
	if err != nil {
		t.Fatalf("ListUserRoles: %v", err)
	}
	if len(rolesResult.Items) != 1 || rolesResult.Items[0].RoleID != roleID {
		t.Fatalf("user roles = %#v", rolesResult.Items)
	}

	permissionsResult, err := service.ListRolePermissions(context.Background(), RolePermissionsQuery{RoleID: roleID})
	if err != nil {
		t.Fatalf("ListRolePermissions: %v", err)
	}
	if len(permissionsResult.Items) != 1 || permissionsResult.Items[0].PermissionID != permissionID {
		t.Fatalf("role permissions = %#v", permissionsResult.Items)
	}
}
