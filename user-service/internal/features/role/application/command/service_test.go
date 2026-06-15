package command

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleCommandServiceCreateRoleDefaultsAndNormalizes(t *testing.T) {
	roles := &stubRoleStore{}
	service := NewRoleCommandService(roles, &stubUserRoleStore{}, &stubRolePermissionStore{}, &stubPermissionLookup{})

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
	roles := &stubRoleStore{role: roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}}
	service := NewRoleCommandService(roles, &stubUserRoleStore{}, &stubRolePermissionStore{}, &stubPermissionLookup{})

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
	roles := &stubRoleStore{role: roledomain.Role{RoleID: roleID, Name: "super_admin", Active: true, IsSystem: true}}
	service := NewRoleCommandService(roles, &stubUserRoleStore{}, &stubRolePermissionStore{}, &stubPermissionLookup{})

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
	roles := &stubRoleStore{rolesByID: map[uuid.UUID]roledomain.Role{
		roleID:      {RoleID: roleID, Name: "operator", Active: true},
		otherRoleID: {RoleID: otherRoleID, Name: "auditor", Active: true},
	}}
	userRoles := &stubUserRoleStore{items: []roledomain.Role{{RoleID: roleID, Name: "operator", Active: true}}}
	service := NewRoleCommandService(roles, userRoles, &stubRolePermissionStore{}, &stubPermissionLookup{})

	result, err := service.AddUserRole(context.Background(), UserRoleCommand{UserID: userID, RoleID: roleID})
	if err != nil {
		t.Fatalf("AddUserRole: %v", err)
	}
	if userRoles.addUserID != userID || userRoles.addRoleID != roleID || len(result.Items) != 1 {
		t.Fatalf("add state = user:%s role:%s result:%#v", userRoles.addUserID, userRoles.addRoleID, result.Items)
	}

	replaced, err := service.ReplaceUserRoles(context.Background(), ReplaceUserRolesCommand{UserID: userID, RoleIDs: []uuid.UUID{roleID, otherRoleID, roleID}})
	if err != nil {
		t.Fatalf("ReplaceUserRoles: %v", err)
	}
	if !reflect.DeepEqual(userRoles.replaceRoleIDs, []uuid.UUID{roleID, otherRoleID}) {
		t.Fatalf("replace role ids = %#v", userRoles.replaceRoleIDs)
	}
	if len(replaced.Items) != 2 {
		t.Fatalf("replaced items = %#v", replaced.Items)
	}
}

func TestRoleCommandServiceRolePermissionBindings(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000006")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000007")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000008")
	roles := &stubRoleStore{role: roledomain.Role{RoleID: roleID, Name: "operator", Active: true}}
	permissions := &stubPermissionLookup{items: map[uuid.UUID]roleapplication.PermissionReference{
		permissionID:      {PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true},
		otherPermissionID: {PermissionID: otherPermissionID, HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true},
	}}
	rolePermissions := &stubRolePermissionStore{items: []roleapplication.PermissionReference{{PermissionID: permissionID, Active: true}}}
	service := NewRoleCommandService(roles, &stubUserRoleStore{}, rolePermissions, permissions)

	result, err := service.AddRolePermission(context.Background(), RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
	if err != nil {
		t.Fatalf("AddRolePermission: %v", err)
	}
	if rolePermissions.addRoleID != roleID || rolePermissions.addPermission.PermissionID != permissionID || len(result.Items) != 1 {
		t.Fatalf("add state = role:%s permission:%#v result:%#v", rolePermissions.addRoleID, rolePermissions.addPermission, result.Items)
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
}

type stubRoleStore struct {
	role            roledomain.Role
	rolesByID       map[uuid.UUID]roledomain.Role
	createInput     roleapplication.CreateRoleInput
	updateCalled    bool
	setActiveCalled bool
}

func (s *stubRoleStore) Create(_ context.Context, input roleapplication.CreateRoleInput) (*roledomain.Role, error) {
	s.createInput = input
	s.role = roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active, IsSystem: input.IsSystem}
	return &s.role, nil
}

func (s *stubRoleStore) GetByRoleID(_ context.Context, roleID uuid.UUID) (*roledomain.Role, error) {
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

func (s *stubRoleStore) List(context.Context, roleapplication.ListRolesInput) ([]roledomain.Role, bool, error) {
	return []roledomain.Role{s.role}, false, nil
}

func (s *stubRoleStore) Update(_ context.Context, input roleapplication.UpdateRoleInput) (*roledomain.Role, error) {
	s.updateCalled = true
	s.role = roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active}
	return &s.role, nil
}

func (s *stubRoleStore) SetActive(_ context.Context, roleID uuid.UUID, active bool) (*roledomain.Role, error) {
	s.setActiveCalled = true
	s.role.RoleID = roleID
	s.role.Active = active
	return &s.role, nil
}

type stubUserRoleStore struct {
	items          []roledomain.Role
	addUserID      uuid.UUID
	addRoleID      uuid.UUID
	replaceRoleIDs []uuid.UUID
}

func (s *stubUserRoleStore) ListByUserID(context.Context, uuid.UUID) ([]roledomain.Role, error) {
	return s.items, nil
}

func (s *stubUserRoleStore) Add(_ context.Context, userID uuid.UUID, roleID uuid.UUID) error {
	s.addUserID = userID
	s.addRoleID = roleID
	return nil
}

func (s *stubUserRoleStore) Replace(_ context.Context, _ uuid.UUID, roleIDs []uuid.UUID) ([]roledomain.Role, error) {
	s.replaceRoleIDs = append([]uuid.UUID(nil), roleIDs...)
	items := make([]roledomain.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		items = append(items, roledomain.Role{RoleID: roleID, Active: true})
	}
	return items, nil
}

func (s *stubUserRoleStore) Remove(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type stubRolePermissionStore struct {
	items              []roleapplication.PermissionReference
	addRoleID          uuid.UUID
	addPermission      roleapplication.PermissionReference
	replacePermissions []roleapplication.PermissionReference
}

func (s *stubRolePermissionStore) ListByRoleID(context.Context, uuid.UUID) ([]roleapplication.PermissionReference, error) {
	return s.items, nil
}

func (s *stubRolePermissionStore) Add(_ context.Context, roleID uuid.UUID, permission roleapplication.PermissionReference) error {
	s.addRoleID = roleID
	s.addPermission = permission
	return nil
}

func (s *stubRolePermissionStore) Replace(_ context.Context, _ uuid.UUID, permissions []roleapplication.PermissionReference) ([]roleapplication.PermissionReference, error) {
	s.replacePermissions = append([]roleapplication.PermissionReference(nil), permissions...)
	return permissions, nil
}

func (s *stubRolePermissionStore) Remove(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type stubPermissionLookup struct {
	items map[uuid.UUID]roleapplication.PermissionReference
	calls []uuid.UUID
}

func (s *stubPermissionLookup) GetActiveByPermissionID(_ context.Context, permissionID uuid.UUID) (*roleapplication.PermissionReference, error) {
	s.calls = append(s.calls, permissionID)
	permission, ok := s.items[permissionID]
	if !ok {
		return nil, errors.New("permission not found")
	}
	return &permission, nil
}
