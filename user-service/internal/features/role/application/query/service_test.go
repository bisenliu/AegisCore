package query

import (
	"context"
	"testing"

	"github.com/google/uuid"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleQueryServiceListRolesNormalizesLimitAndNextCursor(t *testing.T) {
	firstRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")
	lastRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000102")
	roles := &stubRoleStore{items: []roledomain.Role{{RoleID: firstRoleID, Name: "operator", Active: true}, {RoleID: lastRoleID, Name: "auditor", Active: true}}, hasNext: true}
	service := NewRoleQueryService(roles, &stubUserRoleStore{}, &stubRolePermissionStore{})

	result, err := service.ListRoles(context.Background(), ListRolesQuery{})
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if roles.listInput.Limit <= 0 || result.PageSize != roles.listInput.Limit {
		t.Fatalf("page size = %d, store limit = %d", result.PageSize, roles.listInput.Limit)
	}
	if !result.HasNext || result.NextCursor != lastRoleID.String() {
		t.Fatalf("pagination result = %#v", result)
	}
}

func TestRoleQueryServiceGetAndBindingQueriesPassThrough(t *testing.T) {
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000103")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000104")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000105")
	roles := &stubRoleStore{role: roledomain.Role{RoleID: roleID, Name: "operator", Active: true}}
	userRoles := &stubUserRoleStore{items: []roledomain.Role{{RoleID: roleID, Name: "operator", Active: true}}}
	rolePermissions := &stubRolePermissionStore{items: []roleapplication.PermissionReference{{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}}}
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
	if len(rolesResult.Items) != 1 || rolesResult.Items[0].RoleID != roleID || userRoles.listUserID != userID {
		t.Fatalf("user roles = %#v user id = %s", rolesResult.Items, userRoles.listUserID)
	}

	permissionsResult, err := service.ListRolePermissions(context.Background(), RolePermissionsQuery{RoleID: roleID})
	if err != nil {
		t.Fatalf("ListRolePermissions: %v", err)
	}
	if len(permissionsResult.Items) != 1 || permissionsResult.Items[0].PermissionID != permissionID || rolePermissions.listRoleID != roleID {
		t.Fatalf("role permissions = %#v role id = %s", permissionsResult.Items, rolePermissions.listRoleID)
	}
}

type stubRoleStore struct {
	role      roledomain.Role
	items     []roledomain.Role
	hasNext   bool
	listInput roleapplication.ListRolesInput
}

func (s *stubRoleStore) Create(context.Context, roleapplication.CreateRoleInput) (*roledomain.Role, error) {
	return nil, nil
}

func (s *stubRoleStore) GetByRoleID(context.Context, uuid.UUID) (*roledomain.Role, error) {
	return &s.role, nil
}

func (s *stubRoleStore) List(_ context.Context, input roleapplication.ListRolesInput) ([]roledomain.Role, bool, error) {
	s.listInput = input
	return s.items, s.hasNext, nil
}

func (s *stubRoleStore) Update(context.Context, roleapplication.UpdateRoleInput) (*roledomain.Role, error) {
	return nil, nil
}

func (s *stubRoleStore) SetActive(context.Context, uuid.UUID, bool) (*roledomain.Role, error) {
	return nil, nil
}

type stubUserRoleStore struct {
	items      []roledomain.Role
	listUserID uuid.UUID
}

func (s *stubUserRoleStore) ListByUserID(_ context.Context, userID uuid.UUID) ([]roledomain.Role, error) {
	s.listUserID = userID
	return s.items, nil
}

func (s *stubUserRoleStore) Add(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (s *stubUserRoleStore) Replace(context.Context, uuid.UUID, []uuid.UUID) ([]roledomain.Role, error) {
	return nil, nil
}

func (s *stubUserRoleStore) Remove(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type stubRolePermissionStore struct {
	items      []roleapplication.PermissionReference
	listRoleID uuid.UUID
}

func (s *stubRolePermissionStore) ListByRoleID(_ context.Context, roleID uuid.UUID) ([]roleapplication.PermissionReference, error) {
	s.listRoleID = roleID
	return s.items, nil
}

func (s *stubRolePermissionStore) Add(context.Context, uuid.UUID, roleapplication.PermissionReference) error {
	return nil
}

func (s *stubRolePermissionStore) Replace(context.Context, uuid.UUID, []roleapplication.PermissionReference) ([]roleapplication.PermissionReference, error) {
	return nil, nil
}

func (s *stubRolePermissionStore) Remove(context.Context, uuid.UUID, uuid.UUID) error { return nil }
