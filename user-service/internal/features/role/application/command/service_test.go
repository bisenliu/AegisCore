package command

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleCommandServiceCreateRoleDefaultsAndNormalizes(t *testing.T) {
	roles := &roleTestStore{}
	service := NewRoleCommandService(RoleCommandParams{Roles: roles, UserRoles: &userRoleTestStore{}, RolePermissions: &rolePermissionTestStore{}, Permissions: &permissionLookupTestStore{}, PolicyChanges: &recordingRolePolicyChangeNotifier{}})

	result, err := service.CreateRole(context.Background(), CreateRoleCommand{Name: "  Operator  ", Description: "  Ops user  ", IsSystem: true})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if result.Role.RoleID == uuid.Nil {
		t.Fatalf("role id is nil")
	}
	if !result.Role.Active || !result.Role.IsSystem {
		t.Fatalf("role flags = active:%v system:%v", result.Role.Active, result.Role.IsSystem)
	}
	if roles.createInput.Name != "Operator" || roles.createInput.Description != "Ops user" {
		t.Fatalf("create input = %#v", roles.createInput)
	}
}

func TestRoleCommandServiceUpdateRoleProtectsSystemRole(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	roles := &roleTestStore{role: roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}}
	service := NewRoleCommandService(RoleCommandParams{Roles: roles, UserRoles: &userRoleTestStore{}, RolePermissions: &rolePermissionTestStore{}, Permissions: &permissionLookupTestStore{}, PolicyChanges: &recordingRolePolicyChangeNotifier{}})

	_, err := service.UpdateRole(context.Background(), UpdateRoleCommand{RoleID: roleID, Name: "renamed", Description: "system", Active: true})
	if !errors.Is(err, roledomain.ErrSystemRoleProtected) {
		t.Fatalf("err = %v, want ErrSystemRoleProtected", err)
	}
	if roles.updateCalled {
		t.Fatalf("Update called for protected system role")
	}
}

func TestRoleCommandServiceSetRoleActiveProtectsSystemRole(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	roles := &roleTestStore{role: roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}}
	service := NewRoleCommandService(RoleCommandParams{Roles: roles, UserRoles: &userRoleTestStore{}, RolePermissions: &rolePermissionTestStore{}, Permissions: &permissionLookupTestStore{}, PolicyChanges: &recordingRolePolicyChangeNotifier{}})

	_, err := service.SetRoleActive(context.Background(), SetRoleActiveCommand{RoleID: roleID, Active: false})
	if !errors.Is(err, roledomain.ErrSystemRoleProtected) {
		t.Fatalf("err = %v, want ErrSystemRoleProtected", err)
	}
	if roles.setActiveCalled {
		t.Fatalf("SetActive called for protected system role")
	}
}

func TestRoleCommandServiceUserRoleBindings(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000003")
	otherRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000004")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000005")
	roles := &roleTestStore{rolesByID: map[uuid.UUID]roledomain.Role{
		roleID:      {RoleID: roleID, Name: "operator", Active: true},
		otherRoleID: {RoleID: otherRoleID, Name: "auditor", Active: true},
	}}
	userRoles := &userRoleTestStore{items: []roledomain.Role{{RoleID: roleID, Name: "operator", Active: true}}}
	notifier := &recordingRolePolicyChangeNotifier{}
	service := NewRoleCommandService(RoleCommandParams{Roles: roles, UserRoles: userRoles, RolePermissions: &rolePermissionTestStore{}, Permissions: &permissionLookupTestStore{}, PolicyChanges: notifier})

	result, err := service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
	if err != nil {
		t.Fatalf("AddUserRole: %v", err)
	}
	if userRoles.addUserID != userID || userRoles.addRoleID != roleID || len(result.Items) != 1 {
		t.Fatalf("add state = user:%s role:%s result:%#v", userRoles.addUserID, userRoles.addRoleID, result.Items)
	}
	if notifier.reasons[0] != "user_role_added" {
		t.Fatalf("notifier = %#v", notifier.reasons)
	}
	if notifier.changes[0].Kind != permissionapplication.PolicyChangeKindUserRole || notifier.changes[0].UserID != userID || notifier.changes[0].RoleID != roleID {
		t.Fatalf("user role change = %#v", notifier.changes[0])
	}

	replaced, err := service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID, otherRoleID, roleID}})
	if err != nil {
		t.Fatalf("ReplaceUserRoles: %v", err)
	}
	if !reflect.DeepEqual(userRoles.replaceRoleIDs, []uuid.UUID{roleID, otherRoleID}) {
		t.Fatalf("replace role ids = %#v", userRoles.replaceRoleIDs)
	}
	if len(roles.batchLookupRoleIDs) != 1 || !reflect.DeepEqual(roles.batchLookupRoleIDs[0], []uuid.UUID{roleID, otherRoleID}) {
		t.Fatalf("batch lookup role ids = %#v", roles.batchLookupRoleIDs)
	}
	if len(replaced.Items) != 2 {
		t.Fatalf("replaced items = %#v", replaced.Items)
	}
	if notifier.reasons[1] != "user_roles_replaced" {
		t.Fatalf("notifier = %#v", notifier.reasons)
	}
	if notifier.changes[1].Kind != permissionapplication.PolicyChangeKindUserRole || notifier.changes[1].UserID != userID || notifier.changes[1].RoleID != uuid.Nil {
		t.Fatalf("replace user role change = %#v", notifier.changes[1])
	}
}

func TestRoleCommandServiceRolePermissionBindings(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000006")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000007")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000008")
	roles := &roleTestStore{role: roledomain.Role{RoleID: roleID, Name: "operator", Active: true}}
	permissions := &permissionLookupTestStore{items: map[uuid.UUID]roleapplication.PermissionReference{
		permissionID:      {PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true},
		otherPermissionID: {PermissionID: otherPermissionID, HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true},
	}}
	rolePermissions := &rolePermissionTestStore{items: []roleapplication.PermissionReference{{PermissionID: permissionID, Active: true}}}
	notifier := &recordingRolePolicyChangeNotifier{}
	service := NewRoleCommandService(RoleCommandParams{Roles: roles, UserRoles: &userRoleTestStore{}, RolePermissions: rolePermissions, Permissions: permissions, PolicyChanges: notifier})

	result, err := service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
	if err != nil {
		t.Fatalf("AddRolePermission: %v", err)
	}
	if rolePermissions.addRoleID != roleID || rolePermissions.addPermission.PermissionID != permissionID || len(result.Items) != 1 {
		t.Fatalf("add state = role:%s permission:%#v result:%#v", rolePermissions.addRoleID, rolePermissions.addPermission, result.Items)
	}
	if notifier.reasons[0] != "role_permission_added" {
		t.Fatalf("notifier = %#v", notifier.reasons)
	}
	if notifier.changes[0].Kind != permissionapplication.PolicyChangeKindPolicy {
		t.Fatalf("role permission change = %#v", notifier.changes[0])
	}

	replaced, err := service.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: []uuid.UUID{permissionID, otherPermissionID, permissionID}})
	if err != nil {
		t.Fatalf("ReplaceRolePermissions: %v", err)
	}
	if len(permissions.calls) != 3 {
		t.Fatalf("permission lookup calls = %#v", permissions.calls)
	}
	if got := rolePermissions.replacePermissions; len(got) != 2 || got[0].PermissionID != permissionID || got[1].PermissionID != otherPermissionID {
		t.Fatalf("replace permissions = %#v", got)
	}
	if len(replaced.Items) != 2 {
		t.Fatalf("replaced items = %#v", replaced.Items)
	}
	if notifier.reasons[1] != "role_permissions_replaced" {
		t.Fatalf("notifier = %#v", notifier.reasons)
	}
}

func TestRoleCommandServiceSwallowsRefreshFailureAfterSuccessfulWrite(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000009")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000010")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
	refreshErr := errors.New("refresh failed")

	tests := []struct {
		name       string
		run        func(*testing.T, RoleCommandService) any
		wantReason string
	}{
		{
			name: "update role",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.UpdateRole(context.Background(), UpdateRoleCommand{RoleID: roleID, Name: "operator", Active: true})
				if err != nil {
					t.Fatalf("UpdateRole: %v", err)
				}
				return result
			},
			wantReason: "role_updated",
		},
		{
			name: "set role active",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.SetRoleActive(context.Background(), SetRoleActiveCommand{RoleID: roleID, Active: false})
				if err != nil {
					t.Fatalf("SetRoleActive: %v", err)
				}
				return result
			},
			wantReason: "role_active_changed",
		},
		{
			name: "add user role",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
				if err != nil {
					t.Fatalf("AddUserRole: %v", err)
				}
				return result
			},
			wantReason: "user_role_added",
		},
		{
			name: "replace user roles",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID}})
				if err != nil {
					t.Fatalf("ReplaceUserRoles: %v", err)
				}
				return result
			},
			wantReason: "user_roles_replaced",
		},
		{
			name: "remove user role",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.RemoveUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
				if err != nil {
					t.Fatalf("RemoveUserRole: %v", err)
				}
				return result
			},
			wantReason: "user_role_removed",
		},
		{
			name: "add role permission",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
				if err != nil {
					t.Fatalf("AddRolePermission: %v", err)
				}
				return result
			},
			wantReason: "role_permission_added",
		},
		{
			name: "replace role permissions",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: []uuid.UUID{permissionID}})
				if err != nil {
					t.Fatalf("ReplaceRolePermissions: %v", err)
				}
				return result
			},
			wantReason: "role_permissions_replaced",
		},
		{
			name: "remove role permission",
			run: func(t *testing.T, service RoleCommandService) any {
				t.Helper()
				result, err := service.RemoveRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
				if err != nil {
					t.Fatalf("RemoveRolePermission: %v", err)
				}
				return result
			},
			wantReason: "role_permission_removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roles := &roleTestStore{role: roledomain.Role{RoleID: roleID, Name: "operator", Active: true}}
			permissions := &permissionLookupTestStore{items: map[uuid.UUID]roleapplication.PermissionReference{
				permissionID: {PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true},
			}}
			notifier := &recordingRolePolicyChangeNotifier{err: refreshErr}
			service := NewRoleCommandService(RoleCommandParams{Roles: roles, UserRoles: &userRoleTestStore{}, RolePermissions: &rolePermissionTestStore{}, Permissions: permissions, PolicyChanges: notifier})

			result := tt.run(t, service)

			if result == nil {
				t.Fatalf("result is nil")
			}
			if len(notifier.reasons) != 1 || notifier.reasons[0] != tt.wantReason {
				t.Fatalf("notifier = %#v", notifier.reasons)
			}
		})
	}
}

type recordingRolePolicyChangeNotifier struct {
	reasons []string
	changes []permissionapplication.PolicyChange
	err     error
}

func (n *recordingRolePolicyChangeNotifier) NotifyPolicyChanged(_ context.Context, change permissionapplication.PolicyChange) error {
	n.reasons = append(n.reasons, change.Reason)
	n.changes = append(n.changes, change)
	return n.err
}

type roleTestStore struct {
	role               roledomain.Role
	rolesByID          map[uuid.UUID]roledomain.Role
	createInput        roleapplication.CreateRoleInput
	updateCalled       bool
	setActiveCalled    bool
	batchLookupRoleIDs [][]uuid.UUID
}

func (s *roleTestStore) Create(_ context.Context, input roleapplication.CreateRoleInput) (*roledomain.Role, error) {
	s.createInput = input
	s.role = roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active, IsSystem: input.IsSystem}
	return &s.role, nil
}

func (s *roleTestStore) GetByRoleID(_ context.Context, roleID uuid.UUID) (*roledomain.Role, error) {
	if s.rolesByID != nil {
		role, ok := s.rolesByID[roleID]
		if !ok {
			return nil, roledomain.ErrRoleNotFound
		}
		return &role, nil
	}
	if s.role.RoleID != uuid.Nil && s.role.RoleID != roleID {
		return nil, roledomain.ErrRoleNotFound
	}
	return &s.role, nil
}

func (s *roleTestStore) GetByRoleIDs(_ context.Context, roleIDs []uuid.UUID) ([]roledomain.Role, error) {
	s.batchLookupRoleIDs = append(s.batchLookupRoleIDs, append([]uuid.UUID(nil), roleIDs...))
	roles := make([]roledomain.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		role, err := s.GetByRoleID(context.Background(), roleID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}
	return roles, nil
}

func (s *roleTestStore) List(context.Context, roleapplication.ListRolesInput) ([]roledomain.Role, bool, error) {
	return []roledomain.Role{s.role}, false, nil
}

func (s *roleTestStore) Update(_ context.Context, input roleapplication.UpdateRoleInput) (*roledomain.Role, error) {
	s.updateCalled = true
	s.role = roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active}
	return &s.role, nil
}

func (s *roleTestStore) SetActive(_ context.Context, roleID uuid.UUID, active bool) (*roledomain.Role, error) {
	s.setActiveCalled = true
	s.role.RoleID = roleID
	s.role.Active = active
	return &s.role, nil
}

type userRoleTestStore struct {
	items          []roledomain.Role
	addUserID      uuid.UUID
	addRoleID      uuid.UUID
	replaceRoleIDs []uuid.UUID
}

func (s *userRoleTestStore) ListByUserID(context.Context, uuid.UUID) ([]roledomain.Role, error) {
	return s.items, nil
}

func (s *userRoleTestStore) Add(_ context.Context, userID uuid.UUID, roleID uuid.UUID) error {
	s.addUserID = userID
	s.addRoleID = roleID
	return nil
}

func (s *userRoleTestStore) Replace(_ context.Context, _ uuid.UUID, roleIDs []uuid.UUID) ([]roledomain.Role, error) {
	s.replaceRoleIDs = append([]uuid.UUID(nil), roleIDs...)
	items := make([]roledomain.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		items = append(items, roledomain.Role{RoleID: roleID, Active: true})
	}
	return items, nil
}

func (s *userRoleTestStore) Remove(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type rolePermissionTestStore struct {
	items              []roleapplication.PermissionReference
	addRoleID          uuid.UUID
	addPermission      roleapplication.PermissionReference
	replacePermissions []roleapplication.PermissionReference
}

func (s *rolePermissionTestStore) ListByRoleID(context.Context, uuid.UUID) ([]roleapplication.PermissionReference, error) {
	return s.items, nil
}

func (s *rolePermissionTestStore) Add(_ context.Context, roleID uuid.UUID, permission roleapplication.PermissionReference) error {
	s.addRoleID = roleID
	s.addPermission = permission
	return nil
}

func (s *rolePermissionTestStore) Replace(_ context.Context, _ uuid.UUID, permissions []roleapplication.PermissionReference) ([]roleapplication.PermissionReference, error) {
	s.replacePermissions = append([]roleapplication.PermissionReference(nil), permissions...)
	return permissions, nil
}

func (s *rolePermissionTestStore) Remove(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type permissionLookupTestStore struct {
	items map[uuid.UUID]roleapplication.PermissionReference
	calls []uuid.UUID
}

func (s *permissionLookupTestStore) GetActiveByPermissionID(_ context.Context, permissionID uuid.UUID) (*roleapplication.PermissionReference, error) {
	s.calls = append(s.calls, permissionID)
	permission, ok := s.items[permissionID]
	if !ok {
		return nil, errors.New("permission not found")
	}
	return &permission, nil
}
