package seed

import (
	"context"
	"testing"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestSeedServiceDefaultEnsureAndRepeat(t *testing.T) {
	stores := newSeedTestStores()
	service := NewService(stores.roles, stores.permissions, stores.rolePermissions, stores.userRoles)
	permissionCount := len(rbacbaseline.DefaultPermissions())

	result, err := service.Seed(context.Background(), SeedOptions{})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if result.RolesInserted != 1 || result.PermissionsInserted != permissionCount || result.RolePermissionBindingsAdd != permissionCount || stores.rolePermissions.syncCalled {
		t.Fatalf("first result=%#v syncCalled=%v", result, stores.rolePermissions.syncCalled)
	}
	if stores.roles.lastReactivate || stores.permissions.lastReactivate {
		t.Fatal("default seed unexpectedly requested reactivation")
	}
	assertSuperAdminSeedBindings(t, stores.rolePermissions.bindings)

	result, err = service.Seed(context.Background(), SeedOptions{})
	if err != nil {
		t.Fatalf("Seed repeat: %v", err)
	}
	if result.RolesInserted != 0 || result.RolesUpdated != 1 || result.PermissionsInserted != 0 || result.PermissionsUpdated != permissionCount || result.RolePermissionBindingsAdd != 0 {
		t.Fatalf("repeat result=%#v", result)
	}
}

func TestSeedServiceReactivateAndSyncOptions(t *testing.T) {
	stores := newSeedTestStores()
	service := NewService(stores.roles, stores.permissions, stores.rolePermissions, stores.userRoles)

	_, err := service.Seed(context.Background(), SeedOptions{ReactivateSystem: true, SyncSystemBindings: true})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if !stores.roles.lastReactivate || !stores.permissions.lastReactivate {
		t.Fatal("reactivate option was not propagated")
	}
	if !stores.rolePermissions.syncCalled {
		t.Fatal("sync option did not call SyncSystemBindings")
	}
}

func TestSeedServiceAssignSuperAdmin(t *testing.T) {
	stores := newSeedTestStores()
	service := NewService(stores.roles, stores.permissions, stores.rolePermissions, stores.userRoles)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")

	result, err := service.AssignSuperAdmin(context.Background(), userID)
	if err != nil {
		t.Fatalf("AssignSuperAdmin: %v", err)
	}
	if !result.Added {
		t.Fatal("first assignment Added=false")
	}
	if stores.userRoles.lastRoleID.String() != rbacbaseline.SuperAdminRoleID {
		t.Fatalf("roleID = %s", stores.userRoles.lastRoleID)
	}

	result, err = service.AssignSuperAdmin(context.Background(), userID)
	if err != nil {
		t.Fatalf("AssignSuperAdmin repeat: %v", err)
	}
	if result.Added {
		t.Fatal("repeat assignment Added=true")
	}
}

type seedTestStores struct {
	permissions     *seedPermissionTestStore
	roles           *seedRoleTestStore
	rolePermissions *seedRolePermissionTestStore
	userRoles       *seedUserRoleTestStore
}

func newSeedTestStores() seedTestStores {
	return seedTestStores{permissions: &seedPermissionTestStore{items: map[uuid.UUID]permissiondomain.Permission{}}, roles: &seedRoleTestStore{items: map[uuid.UUID]roledomain.Role{}}, rolePermissions: &seedRolePermissionTestStore{bindings: map[uuid.UUID]map[uuid.UUID]struct{}{}}, userRoles: &seedUserRoleTestStore{bindings: map[uuid.UUID]map[uuid.UUID]struct{}{}}}
}

type seedRoleTestStore struct {
	items          map[uuid.UUID]roledomain.Role
	lastReactivate bool
}

func (s *seedRoleTestStore) UpsertSystemRole(_ context.Context, input roleapplication.SeedRoleInput) (*roledomain.Role, bool, error) {
	s.lastReactivate = input.ReactivateSystem
	role := roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active, IsSystem: input.IsSystem}
	_, exists := s.items[input.RoleID]
	s.items[input.RoleID] = role
	return &role, !exists, nil
}

type seedPermissionTestStore struct {
	items          map[uuid.UUID]permissiondomain.Permission
	lastReactivate bool
}

func (s *seedPermissionTestStore) UpsertSystemPermission(_ context.Context, input permissionapplication.SeedPermissionInput) (*permissiondomain.Permission, bool, error) {
	s.lastReactivate = input.ReactivateSystem
	permission := permissiondomain.Permission{PermissionID: input.PermissionID, Name: input.Name, Description: input.Description, Module: input.Module, HTTPMethod: input.HTTPMethod, PathTemplate: input.PathTemplate, Active: input.Active, IsSystem: input.IsSystem}
	_, exists := s.items[input.PermissionID]
	s.items[input.PermissionID] = permission
	return &permission, !exists, nil
}

type seedRolePermissionTestStore struct {
	bindings   map[uuid.UUID]map[uuid.UUID]struct{}
	syncCalled bool
}

func (s *seedRolePermissionTestStore) EnsureSystemBindings(_ context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, error) {
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

func (s *seedRolePermissionTestStore) SyncSystemBindings(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (int, int, error) {
	s.syncCalled = true
	added, err := s.EnsureSystemBindings(ctx, roleID, permissionIDs)
	return added, 0, err
}

type seedUserRoleTestStore struct {
	bindings   map[uuid.UUID]map[uuid.UUID]struct{}
	lastRoleID uuid.UUID
}

func (s *seedUserRoleTestStore) AssignRole(_ context.Context, userID uuid.UUID, roleID uuid.UUID) (bool, error) {
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

func assertSuperAdminSeedBindings(t *testing.T, bindings map[uuid.UUID]map[uuid.UUID]struct{}) {
	t.Helper()
	roleID := uuid.MustParse(rbacbaseline.SuperAdminRoleID)
	actual := bindings[roleID]
	if len(actual) != len(rbacbaseline.DefaultPermissions()) {
		t.Fatalf("super admin bindings = %d, want %d", len(actual), len(rbacbaseline.DefaultPermissions()))
	}
	for _, permission := range rbacbaseline.DefaultPermissions() {
		permissionID := uuid.MustParse(permission.PermissionID)
		if _, ok := actual[permissionID]; !ok {
			t.Fatalf("super admin seed binding missing permission_id %s", permission.PermissionID)
		}
	}
}
