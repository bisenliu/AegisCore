package seed

import (
	"context"
	"testing"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecatalog "github.com/aegiscore/user-service/internal/features/role/application/catalog"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestSeedServiceDefaultEnsureAndRepeat(t *testing.T) {
	fakes := newSeedFakes()
	service := NewService(fakes.roles, fakes.permissions, fakes.rolePermissions, fakes.userRoles)

	result, err := service.Seed(context.Background(), SeedOptions{})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if result.RolesInserted != 1 || result.PermissionsInserted != 3 || result.RolePermissionBindingsAdd != 3 || fakes.rolePermissions.syncCalled {
		t.Fatalf("first result=%#v syncCalled=%v", result, fakes.rolePermissions.syncCalled)
	}
	if fakes.roles.lastReactivate || fakes.permissions.lastReactivate {
		t.Fatal("default seed unexpectedly requested reactivation")
	}

	result, err = service.Seed(context.Background(), SeedOptions{})
	if err != nil {
		t.Fatalf("Seed repeat: %v", err)
	}
	if result.RolesInserted != 0 || result.RolesUpdated != 1 || result.PermissionsInserted != 0 || result.PermissionsUpdated != 3 || result.RolePermissionBindingsAdd != 0 {
		t.Fatalf("repeat result=%#v", result)
	}
}

func TestSeedServiceReactivateAndSyncOptions(t *testing.T) {
	fakes := newSeedFakes()
	service := NewService(fakes.roles, fakes.permissions, fakes.rolePermissions, fakes.userRoles)

	_, err := service.Seed(context.Background(), SeedOptions{ReactivateSystem: true, SyncSystemBindings: true})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if !fakes.roles.lastReactivate || !fakes.permissions.lastReactivate {
		t.Fatal("reactivate option was not propagated")
	}
	if !fakes.rolePermissions.syncCalled {
		t.Fatal("sync option did not call SyncSystemBindings")
	}
}

func TestSeedServiceAssignSuperAdmin(t *testing.T) {
	fakes := newSeedFakes()
	service := NewService(fakes.roles, fakes.permissions, fakes.rolePermissions, fakes.userRoles)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")

	result, err := service.AssignSuperAdmin(context.Background(), userID)
	if err != nil {
		t.Fatalf("AssignSuperAdmin: %v", err)
	}
	if !result.Added {
		t.Fatal("first assignment Added=false")
	}
	if fakes.userRoles.lastRoleID.String() != rolecatalog.SuperAdminRoleID {
		t.Fatalf("roleID = %s", fakes.userRoles.lastRoleID)
	}

	result, err = service.AssignSuperAdmin(context.Background(), userID)
	if err != nil {
		t.Fatalf("AssignSuperAdmin repeat: %v", err)
	}
	if result.Added {
		t.Fatal("repeat assignment Added=true")
	}
}

type seedFakes struct {
	permissions     *fakeSeedPermissionStore
	roles           *fakeSeedRoleStore
	rolePermissions *fakeSeedRolePermissionStore
	userRoles       *fakeSeedUserRoleStore
}

func newSeedFakes() seedFakes {
	return seedFakes{permissions: &fakeSeedPermissionStore{items: map[uuid.UUID]permissiondomain.Permission{}}, roles: &fakeSeedRoleStore{items: map[uuid.UUID]roledomain.Role{}}, rolePermissions: &fakeSeedRolePermissionStore{bindings: map[uuid.UUID]map[uuid.UUID]struct{}{}}, userRoles: &fakeSeedUserRoleStore{bindings: map[uuid.UUID]map[uuid.UUID]struct{}{}}}
}

type fakeSeedRoleStore struct {
	items          map[uuid.UUID]roledomain.Role
	lastReactivate bool
}

func (s *fakeSeedRoleStore) UpsertSystemRole(_ context.Context, input roleapplication.SeedRoleInput) (*roledomain.Role, bool, error) {
	s.lastReactivate = input.ReactivateSystem
	role := roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active, IsSystem: input.IsSystem}
	_, exists := s.items[input.RoleID]
	s.items[input.RoleID] = role
	return &role, !exists, nil
}

type fakeSeedPermissionStore struct {
	items          map[uuid.UUID]permissiondomain.Permission
	lastReactivate bool
}

func (s *fakeSeedPermissionStore) UpsertSystemPermission(_ context.Context, input permissionapplication.SeedPermissionInput) (*permissiondomain.Permission, bool, error) {
	s.lastReactivate = input.ReactivateSystem
	permission := permissiondomain.Permission{PermissionID: input.PermissionID, Name: input.Name, Description: input.Description, Module: input.Module, HTTPMethod: input.HTTPMethod, PathTemplate: input.PathTemplate, Active: input.Active, IsSystem: input.IsSystem}
	_, exists := s.items[input.PermissionID]
	s.items[input.PermissionID] = permission
	return &permission, !exists, nil
}

type fakeSeedRolePermissionStore struct {
	bindings   map[uuid.UUID]map[uuid.UUID]struct{}
	syncCalled bool
}

func (s *fakeSeedRolePermissionStore) EnsureSystemBindings(_ context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, error) {
	if s.bindings[roleID] == nil {
		s.bindings[roleID] = map[uuid.UUID]struct{}{}
	}
	added := 0
	for _, permissionID := range permissionIDs {
		if _, ok := s.bindings[roleID][permissionID]; ok {
			continue
		}
		s.bindings[roleID][permissionID] = struct{}{}
		added++
	}
	return added, nil
}

func (s *fakeSeedRolePermissionStore) SyncSystemBindings(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, int, error) {
	s.syncCalled = true
	added, err := s.EnsureSystemBindings(ctx, roleID, permissionIDs)
	return added, 0, err
}

type fakeSeedUserRoleStore struct {
	bindings   map[uuid.UUID]map[uuid.UUID]struct{}
	lastRoleID uuid.UUID
}

func (s *fakeSeedUserRoleStore) AssignRole(_ context.Context, userID uuid.UUID, roleID uuid.UUID) (bool, error) {
	s.lastRoleID = roleID
	if s.bindings[userID] == nil {
		s.bindings[userID] = map[uuid.UUID]struct{}{}
	}
	if _, ok := s.bindings[userID][roleID]; ok {
		return false, nil
	}
	s.bindings[userID][roleID] = struct{}{}
	return true, nil
}
