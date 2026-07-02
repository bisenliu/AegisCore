package command

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleCommandServiceCreateRoleDefaultsAndNormalizes(t *testing.T) {
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input roleapplication.CreateRoleInput) (*roledomain.Role, error) {
		if input.RoleID == uuid.Nil {
			t.Fatalf("role id is nil")
		}
		if input.Name != "Operator" || input.Description != "Ops user" || !input.Active || !input.IsSystem {
			t.Fatalf("create input = %#v", input)
		}
		return &roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active, IsSystem: input.IsSystem}, nil
	})

	result, err := fixture.service.CreateRole(context.Background(), CreateRoleCommand{Name: "  Operator  ", Description: "  Ops user  ", IsSystem: true})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if result.Role.RoleID == uuid.Nil {
		t.Fatalf("role id is nil")
	}
	if !result.Role.Active || !result.Role.IsSystem {
		t.Fatalf("role flags = active:%v system:%v", result.Role.Active, result.Role.IsSystem)
	}
	if result.Role.Name != "Operator" || result.Role.Description != "Ops user" {
		t.Fatalf("role = %#v", result.Role)
	}
}

func TestRoleCommandServiceUpdateRoleProtectsSystemRole(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}, nil)

	_, err := fixture.service.UpdateRole(context.Background(), UpdateRoleCommand{RoleID: roleID, Name: "renamed", Description: "system", Active: true})
	if !errors.Is(err, roledomain.ErrSystemRoleProtected) {
		t.Fatalf("err = %v, want ErrSystemRoleProtected", err)
	}
}

func TestRoleCommandServiceSetRoleActiveProtectsSystemRole(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	fixture := newRoleCommandFixture(t)
	fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}, nil)

	_, err := fixture.service.SetRoleActive(context.Background(), SetRoleActiveCommand{RoleID: roleID, Active: false})
	if !errors.Is(err, roledomain.ErrSystemRoleProtected) {
		t.Fatalf("err = %v, want ErrSystemRoleProtected", err)
	}
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
		fixture.userRoles.EXPECT().Add(gomock.Any(), userID, roleID).Return(nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_role_added", userID, roleID)).Return(nil),
		fixture.userRoles.EXPECT().ListByUserID(gomock.Any(), userID).Return([]roledomain.Role{role}, nil),
	)

	result, err := fixture.service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
	if err != nil {
		t.Fatalf("AddUserRole: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].RoleID != roleID {
		t.Fatalf("add result = %#v", result.Items)
	}

	gomock.InOrder(
		fixture.roles.EXPECT().GetByRoleIDs(gomock.Any(), uuidSliceMatches(roleID, otherRoleID)).Return([]roledomain.Role{role, otherRole}, nil),
		fixture.userRoles.EXPECT().Replace(gomock.Any(), userID, uuidSliceMatches(roleID, otherRoleID)).Return([]roledomain.Role{role, otherRole}, nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_roles_replaced", userID, uuid.Nil)).Return(nil),
	)

	replaced, err := fixture.service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID, otherRoleID, roleID}})
	if err != nil {
		t.Fatalf("ReplaceUserRoles: %v", err)
	}
	if len(replaced.Items) != 2 || replaced.Items[0].RoleID != roleID || replaced.Items[1].RoleID != otherRoleID {
		t.Fatalf("replaced items = %#v", replaced.Items)
	}
}

func TestRoleCommandServiceRolePermissionBindings(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000006")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000007")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000008")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Active: true}
	permission := roleapplication.PermissionReference{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}
	otherPermission := roleapplication.PermissionReference{PermissionID: otherPermissionID, HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true}
	fixture := newRoleCommandFixture(t)

	gomock.InOrder(
		fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
		fixture.permissions.EXPECT().GetActiveByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
		fixture.rolePermissions.EXPECT().Add(gomock.Any(), roleID, permission).Return(nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permission_added", uuid.Nil, uuid.Nil)).Return(nil),
		fixture.rolePermissions.EXPECT().ListByRoleID(gomock.Any(), roleID).Return([]roleapplication.PermissionReference{permission}, nil),
	)

	result, err := fixture.service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
	if err != nil {
		t.Fatalf("AddRolePermission: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].PermissionID != permissionID {
		t.Fatalf("add result = %#v", result.Items)
	}

	gomock.InOrder(
		fixture.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
		fixture.permissions.EXPECT().GetActiveByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
		fixture.permissions.EXPECT().GetActiveByPermissionID(gomock.Any(), otherPermissionID).Return(&otherPermission, nil),
		fixture.rolePermissions.EXPECT().Replace(gomock.Any(), roleID, permissionSliceMatches(permission, otherPermission)).Return([]roleapplication.PermissionReference{permission, otherPermission}, nil),
		fixture.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permissions_replaced", uuid.Nil, uuid.Nil)).Return(nil),
	)

	replaced, err := fixture.service.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: []uuid.UUID{permissionID, otherPermissionID, permissionID}})
	if err != nil {
		t.Fatalf("ReplaceRolePermissions: %v", err)
	}
	if len(replaced.Items) != 2 || replaced.Items[0].PermissionID != permissionID || replaced.Items[1].PermissionID != otherPermissionID {
		t.Fatalf("replaced items = %#v", replaced.Items)
	}
}

func TestRoleCommandServiceSwallowsRefreshFailureAfterSuccessfulWrite(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000009")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000010")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
	role := roledomain.Role{RoleID: roleID, Name: "operator", Active: true}
	permission := roleapplication.PermissionReference{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}
	refreshErr := errors.New("refresh failed")

	tests := []struct {
		name  string
		setup func(*roleCommandFixture)
		run   func(*testing.T, RoleCommandService) any
	}{
		{
			name: "update role",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.roles.EXPECT().Update(gomock.Any(), gomock.Any()).Return(&role, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_updated", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.UpdateRole(context.Background(), UpdateRoleCommand{RoleID: roleID, Name: "operator", Active: true})
				if err != nil {
					t.Fatalf("UpdateRole: %v", err)
				}
				return result
			},
		},
		{
			name: "set role active",
			setup: func(f *roleCommandFixture) {
				inactiveRole := role
				inactiveRole.Active = false
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.roles.EXPECT().SetActive(gomock.Any(), roleID, false).Return(&inactiveRole, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_active_changed", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.SetRoleActive(context.Background(), SetRoleActiveCommand{RoleID: roleID, Active: false})
				if err != nil {
					t.Fatalf("SetRoleActive: %v", err)
				}
				return result
			},
		},
		{
			name: "add user role",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.userRoles.EXPECT().Add(gomock.Any(), userID, roleID).Return(nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_role_added", userID, roleID)).Return(refreshErr),
					f.userRoles.EXPECT().ListByUserID(gomock.Any(), userID).Return([]roledomain.Role{role}, nil),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
				if err != nil {
					t.Fatalf("AddUserRole: %v", err)
				}
				return result
			},
		},
		{
			name: "replace user roles",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleIDs(gomock.Any(), uuidSliceMatches(roleID)).Return([]roledomain.Role{role}, nil),
					f.userRoles.EXPECT().Replace(gomock.Any(), userID, uuidSliceMatches(roleID)).Return([]roledomain.Role{role}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_roles_replaced", userID, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID}})
				if err != nil {
					t.Fatalf("ReplaceUserRoles: %v", err)
				}
				return result
			},
		},
		{
			name: "remove user role",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.userRoles.EXPECT().Remove(gomock.Any(), userID, roleID).Return(nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindUserRole, "user_role_removed", userID, roleID)).Return(refreshErr),
					f.userRoles.EXPECT().ListByUserID(gomock.Any(), userID).Return([]roledomain.Role{}, nil),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.RemoveUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
				if err != nil {
					t.Fatalf("RemoveUserRole: %v", err)
				}
				return result
			},
		},
		{
			name: "add role permission",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.permissions.EXPECT().GetActiveByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
					f.rolePermissions.EXPECT().Add(gomock.Any(), roleID, permission).Return(nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permission_added", uuid.Nil, uuid.Nil)).Return(refreshErr),
					f.rolePermissions.EXPECT().ListByRoleID(gomock.Any(), roleID).Return([]roleapplication.PermissionReference{permission}, nil),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
				if err != nil {
					t.Fatalf("AddRolePermission: %v", err)
				}
				return result
			},
		},
		{
			name: "replace role permissions",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.roles.EXPECT().GetByRoleID(gomock.Any(), roleID).Return(&role, nil),
					f.permissions.EXPECT().GetActiveByPermissionID(gomock.Any(), permissionID).Return(&permission, nil),
					f.rolePermissions.EXPECT().Replace(gomock.Any(), roleID, permissionSliceMatches(permission)).Return([]roleapplication.PermissionReference{permission}, nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permissions_replaced", uuid.Nil, uuid.Nil)).Return(refreshErr),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: []uuid.UUID{permissionID}})
				if err != nil {
					t.Fatalf("ReplaceRolePermissions: %v", err)
				}
				return result
			},
		},
		{
			name: "remove role permission",
			setup: func(f *roleCommandFixture) {
				gomock.InOrder(
					f.rolePermissions.EXPECT().Remove(gomock.Any(), roleID, permissionID).Return(nil),
					f.policyChanges.EXPECT().NotifyPolicyChanged(gomock.Any(), policyChangeMatches(permissionapplication.PolicyChangeKindPolicy, "role_permission_removed", uuid.Nil, uuid.Nil)).Return(refreshErr),
					f.rolePermissions.EXPECT().ListByRoleID(gomock.Any(), roleID).Return([]roleapplication.PermissionReference{}, nil),
				)
			},
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.RemoveRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
				if err != nil {
					t.Fatalf("RemoveRolePermission: %v", err)
				}
				return result
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newRoleCommandFixture(t)
			tt.setup(fixture)

			result := tt.run(t, fixture.service)
			if result == nil {
				t.Fatalf("result is nil")
			}
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
	fixture.service = NewRoleCommandService(RoleCommandParams{Roles: fixture.roles, UserRoles: fixture.userRoles, RolePermissions: fixture.rolePermissions, Permissions: fixture.permissions, PolicyChanges: fixture.policyChanges})
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
