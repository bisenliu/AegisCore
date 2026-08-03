package command

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleCommandServiceCreateRoleDefaultsAndNormalizes(t *testing.T) {
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input roleapplication.CreateRoleInput, _ roleapplication.PolicyChange) (*roleapplication.RoleWriteResult, error) {
		require.NotEqual(t, uuid.Nil, input.RoleID)
		require.Equal(t, "Operator", input.Name)
		require.Equal(t, "Ops user", input.Description)
		require.True(t, input.Active)
		return &roleapplication.RoleWriteResult{Role: roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active}, Revision: 1}, nil
	})
	fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(1), gomock.Any()).Return(nil)

	result, err := fixture.service.CreateRole(context.Background(), CreateRoleCommand{Name: "  Operator  ", Description: "  Ops user  "})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, result.Role.RoleID)
	require.True(t, result.Role.Active)
	require.False(t, result.Role.IsSystem)
	require.Equal(t, "Operator", result.Role.Name)
	require.Equal(t, "Ops user", result.Role.Description)
}

func TestRoleCommandServiceCreateRoleReturnsStoreError(t *testing.T) {
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRoleAlreadyExists)

	result, err := fixture.service.CreateRole(context.Background(), CreateRoleCommand{Name: "operator"})
	require.ErrorIs(t, err, roledomain.ErrRoleAlreadyExists)
	require.Nil(t, result)
}

func TestRoleCommandServiceUpdateRoleProtectsSystemRole(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}, nil)
	fixture.roles.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := fixture.service.UpdateRole(context.Background(), UpdateRoleCommand{RoleID: roleID, Name: "renamed", Description: "system", Active: true})
	require.ErrorIs(t, err, roledomain.ErrSystemRoleProtected)
}

func TestRoleCommandServiceSetRoleActiveProtectsSystemRole(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}, nil)
	fixture.roles.EXPECT().SetActive(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := fixture.service.SetRoleActive(context.Background(), SetRoleActiveCommand{RoleID: roleID, Active: false})
	require.ErrorIs(t, err, roledomain.ErrSystemRoleProtected)
}

func TestRoleCommandServiceNoopRoleMutationsSkipWriteAndNotify(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000030")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Description: "operations", Active: true}

	t.Run("update", func(t *testing.T) {
		fixture := newRoleCommandFixture(t)
		fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil)
		fixture.roles.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		require.NoError(t, fixture.service.UpdateRole(context.Background(), UpdateRoleCommand{RoleID: roleID, Name: role.Name, Description: role.Description, Active: role.Active}))
	})

	t.Run("status", func(t *testing.T) {
		fixture := newRoleCommandFixture(t)
		fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil)
		fixture.roles.EXPECT().SetActive(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		require.NoError(t, fixture.service.SetRoleActive(context.Background(), SetRoleActiveCommand{RoleID: roleID, Active: role.Active}))
	})
}

func TestRoleCommandServiceUserRoleBindings(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000003")
	otherRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000004")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000005")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Active: true}
	otherRole := roledomain.Role{RoleID: otherRoleID, Name: "auditor", Active: true}
	fixture := newRoleCommandFixture(t)

	gomock.InOrder(
		fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
		fixture.userRoles.EXPECT().Add(gomock.Any(), userID, roleID, gomock.Any()).Return(roleapplication.RolesWriteResult{Items: []roledomain.Role{role}, Revision: 11}, nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(11), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_role_added", userID, roleID)).Return(nil),
	)

	result, err := fixture.service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, roleID, result.Items[0].RoleID)

	gomock.InOrder(
		fixture.roles.EXPECT().GetByRoleIDs(gomock.Any(), uuidSliceMatches(roleID, otherRoleID)).Return([]roledomain.Role{role, otherRole}, nil),
		fixture.userRoles.EXPECT().Replace(gomock.Any(), userID, uuidSliceMatches(roleID, otherRoleID), gomock.Any()).Return(roleapplication.RolesWriteResult{Items: []roledomain.Role{role, otherRole}, Revision: 12}, nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(12), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_roles_replaced", userID, uuid.Nil)).Return(nil),
	)

	replaced, err := fixture.service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID, otherRoleID, roleID}})
	require.NoError(t, err)
	require.Len(t, replaced.Items, 2)
	require.Equal(t, roleID, replaced.Items[0].RoleID)
	require.Equal(t, otherRoleID, replaced.Items[1].RoleID)
}

func TestRoleCommandServiceUserRoleLookupFailureSkipsWriteAndNotify(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000021")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000022")
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(nil, roledomain.ErrRoleNotFound)
	fixture.userRoles.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	result, err := fixture.service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	require.Nil(t, result)
}

func TestRoleCommandServiceReplaceUserRolesLookupFailureSkipsWriteAndNotify(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000023")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000024")
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleIDs(gomock.Any(), uuidSliceMatches(roleID)).Return(nil, roledomain.ErrRoleNotFound)
	fixture.userRoles.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	result, err := fixture.service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID, roleID}})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	require.Nil(t, result)
}

func TestRoleCommandServiceInactiveUserRoleSkipsWriteAndNotify(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000025")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000026")
	role := roledomain.Role{RoleID: roleID, Name: "inactive", Active: false}
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil)
	fixture.userRoles.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	result, err := fixture.service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
	require.ErrorIs(t, err, roledomain.ErrRoleInactive)
	require.Nil(t, result)
}

func TestRoleCommandServiceReplaceInactiveUserRoleSkipsWriteAndNotify(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000027")
	otherRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000028")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000029")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Active: true}
	inactiveRole := roledomain.Role{RoleID: otherRoleID, Name: "inactive", Active: false}
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleIDs(gomock.Any(), uuidSliceMatches(roleID, otherRoleID)).Return([]roledomain.Role{role, inactiveRole}, nil)
	fixture.userRoles.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	result, err := fixture.service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID, otherRoleID}})
	require.ErrorIs(t, err, roledomain.ErrRoleInactive)
	require.Nil(t, result)
}

func TestRoleCommandServiceRolePermissionBindings(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000006")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000007")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000008")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Active: true}
	permission := roleapplication.PermissionReference{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users"}
	otherPermission := roleapplication.PermissionReference{PermissionID: otherPermissionID, HTTPMethod: "POST", PathTemplate: "/api/v1/users"}
	fixture := newRoleCommandFixture(t)

	gomock.InOrder(
		fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
		fixture.permissions.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
		fixture.rolePermissions.EXPECT().Add(gomock.Any(), roleID, permission, gomock.Any()).Return(roleapplication.PermissionsWriteResult{Items: []roleapplication.PermissionReference{permission}, Revision: 21}, nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(21), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permission_added", uuid.Nil, uuid.Nil)).Return(nil),
	)

	result, err := fixture.service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, permissionID, result.Items[0].PermissionID)

	gomock.InOrder(
		fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
		fixture.permissions.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
		fixture.permissions.EXPECT().GetByPermissionID(gomock.Any(), otherPermissionID).Return(&otherPermission, nil),
		fixture.rolePermissions.EXPECT().Replace(gomock.Any(), roleID, permissionSliceMatches(permission, otherPermission), gomock.Any()).Return(roleapplication.PermissionsWriteResult{Items: []roleapplication.PermissionReference{permission, otherPermission}, Revision: 22}, nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(22), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permissions_replaced", uuid.Nil, uuid.Nil)).Return(nil),
	)

	replaced, err := fixture.service.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: []uuid.UUID{permissionID, otherPermissionID, permissionID}})
	require.NoError(t, err)
	require.Len(t, replaced.Items, 2)
	require.Equal(t, permissionID, replaced.Items[0].PermissionID)
	require.Equal(t, otherPermissionID, replaced.Items[1].PermissionID)
}

func TestRoleCommandServiceRolePermissionLookupFailureSkipsWriteAndNotify(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000025")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000026")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000027")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Active: true}
	permission := roleapplication.PermissionReference{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users"}
	lookupErr := errors.Join(permissiondomain.ErrPermissionNotFound, errors.New("permission lookup failed"))

	t.Run("add skips binding write when permission is unavailable", func(t *testing.T) {
		fixture := newRoleCommandFixture(t)
		gomock.InOrder(
			fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
			fixture.permissions.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(nil, lookupErr),
		)
		fixture.rolePermissions.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		result, err := fixture.service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
		require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
		require.Nil(t, result)
	})

	t.Run("replace skips binding write when any permission lookup fails", func(t *testing.T) {
		fixture := newRoleCommandFixture(t)
		gomock.InOrder(
			fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
			fixture.permissions.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
			fixture.permissions.EXPECT().GetByPermissionID(gomock.Any(), otherPermissionID).Return(nil, permissiondomain.ErrPermissionNotFound),
		)
		fixture.rolePermissions.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		result, err := fixture.service.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: []uuid.UUID{permissionID, otherPermissionID, permissionID}})
		require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
		require.Nil(t, result)
	})
}

func TestRoleCommandServiceRequiresPolicyChangeNotifier(t *testing.T) {
	var service RoleCommandService
	require.NotPanics(t, func() {
		var err error
		service, err = NewRoleCommandService(nil, nil, nil, nil, nil)
		require.ErrorContains(t, err, "role policy change notifier is required")
	})
	require.Nil(t, service)
}

func TestRoleCommandServiceReturnsCommittedSuccessAfterRefreshFailure(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000009")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000010")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Active: true}
	permission := roleapplication.PermissionReference{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users"}
	refreshErr := errors.New("refresh failed")

	tests := []struct {
		name  string
		setup func(*roleCommandFixture)
		run   func(RoleCommandService) error
	}{
		{
			name: "update role",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.roles.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(roleapplication.PolicyWriteResult{Revision: 31}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(31), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_updated", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				return service.UpdateRole(context.Background(), UpdateRoleCommand{RoleID: roleID, Name: "operator updated", Active: true})
			},
		},
		{
			name: "set role active",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.roles.EXPECT().SetActive(gomock.Any(), roleID, false, gomock.Any()).Return(roleapplication.PolicyWriteResult{Revision: 32}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(32), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_active_changed", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				return service.SetRoleActive(context.Background(), SetRoleActiveCommand{RoleID: roleID, Active: false})
			},
		},
		{
			name: "add user role",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.userRoles.EXPECT().Add(gomock.Any(), userID, roleID, gomock.Any()).Return(roleapplication.RolesWriteResult{Items: []roledomain.Role{role}, Revision: 33}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(33), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_role_added", userID, roleID)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				result, err := service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
				require.NotNil(t, result)
				require.Equal(t, int64(33), result.Revision)
				return err
			},
		},
		{
			name: "replace user roles",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleIDs(gomock.Any(), uuidSliceMatches(roleID)).Return([]roledomain.Role{role}, nil),
					f.userRoles.EXPECT().Replace(gomock.Any(), userID, uuidSliceMatches(roleID), gomock.Any()).Return(roleapplication.RolesWriteResult{Items: []roledomain.Role{role}, Revision: 34}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(34), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_roles_replaced", userID, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				result, err := service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID}})
				require.NotNil(t, result)
				require.Equal(t, int64(34), result.Revision)
				return err
			},
		},
		{
			name: "remove user role",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.userRoles.EXPECT().Remove(gomock.Any(), userID, roleID, gomock.Any()).Return(roleapplication.RolesWriteResult{Revision: 35}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(35), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_role_removed", userID, roleID)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				result, err := service.RemoveUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
				require.NotNil(t, result)
				require.Equal(t, int64(35), result.Revision)
				return err
			},
		},
		{
			name: "add role permission",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.permissions.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
					f.rolePermissions.EXPECT().Add(gomock.Any(), roleID, permission, gomock.Any()).Return(roleapplication.PermissionsWriteResult{Items: []roleapplication.PermissionReference{permission}, Revision: 36}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(36), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permission_added", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				result, err := service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
				require.NotNil(t, result)
				require.Equal(t, int64(36), result.Revision)
				return err
			},
		},
		{
			name: "replace role permissions",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.permissions.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
					f.rolePermissions.EXPECT().Replace(gomock.Any(), roleID, permissionSliceMatches(permission), gomock.Any()).Return(roleapplication.PermissionsWriteResult{Items: []roleapplication.PermissionReference{permission}, Revision: 37}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(37), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permissions_replaced", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				result, err := service.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: []uuid.UUID{permissionID}})
				require.NotNil(t, result)
				require.Equal(t, int64(37), result.Revision)
				return err
			},
		},
		{
			name: "remove role permission",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.rolePermissions.EXPECT().Remove(gomock.Any(), roleID, permissionID, gomock.Any()).Return(roleapplication.PermissionsWriteResult{Revision: 38}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), int64(38), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permission_removed", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(service RoleCommandService) error {
				result, err := service.RemoveRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
				require.NotNil(t, result)
				require.Equal(t, int64(38), result.Revision)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newRoleCommandFixture(t)
			tt.setup(fixture)

			require.NoError(t, tt.run(fixture.service))
		})
	}
}

type roleCommandFixture struct {
	service         RoleCommandService
	permissions     *MockPermissionLookup
	policyChanges   *MockPolicyChangeNotifier
	rolePermissions *MockRolePermissionStore
	roles           *MockRoleStore
	userRoles       *MockUserRoleStore
}

func newRoleCommandFixture(t testing.TB) *roleCommandFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	fixture := &roleCommandFixture{
		permissions:     NewMockPermissionLookup(ctrl),
		policyChanges:   NewMockPolicyChangeNotifier(ctrl),
		rolePermissions: NewMockRolePermissionStore(ctrl),
		roles:           NewMockRoleStore(ctrl),
		userRoles:       NewMockUserRoleStore(ctrl),
	}
	var err error
	fixture.service, err = NewRoleCommandService(fixture.roles, fixture.userRoles, fixture.rolePermissions, fixture.permissions, fixture.policyChanges)
	require.NoError(t, err)
	return fixture
}

func policyChangeMatches(kind permissionapplication.PolicyChangeKind, reason string, userID uuid.UUID, roleID uuid.UUID) gomock.Matcher {
	return policyChangeMatcher{kind: kind, reason: reason, userID: userID, roleID: roleID}
}

type policyChangeMatcher struct {
	kind   permissionapplication.PolicyChangeKind
	reason string
	userID uuid.UUID
	roleID uuid.UUID
}

func (m policyChangeMatcher) Matches(value any) bool {
	change, ok := value.(permissionapplication.PolicyChange)
	if !ok {
		return false
	}
	return change.Kind == m.kind &&
		change.Reason == m.reason &&
		change.UserID == m.userID &&
		change.RoleID == m.roleID &&
		change.PermissionID == uuid.Nil
}

func (m policyChangeMatcher) String() string {
	return "policy change matching kind, reason, user id, role id, and empty permission id"
}

func uuidSliceMatches(values ...uuid.UUID) gomock.Matcher {
	return uuidSliceMatcher{values: values}
}

type uuidSliceMatcher struct {
	values []uuid.UUID
}

func (m uuidSliceMatcher) Matches(value any) bool {
	got, ok := value.([]uuid.UUID)
	if !ok {
		return false
	}
	return reflect.DeepEqual(got, m.values)
}

func (m uuidSliceMatcher) String() string {
	return "uuid slice matching expected values"
}

func permissionSliceMatches(values ...roleapplication.PermissionReference) gomock.Matcher {
	return permissionSliceMatcher{values: values}
}

type permissionSliceMatcher struct {
	values []roleapplication.PermissionReference
}

func (m permissionSliceMatcher) Matches(value any) bool {
	got, ok := value.([]roleapplication.PermissionReference)
	if !ok {
		return false
	}
	return reflect.DeepEqual(got, m.values)
}

func (m permissionSliceMatcher) String() string {
	return "permission reference slice matching expected values"
}
