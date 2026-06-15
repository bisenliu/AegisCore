package catalog

import (
	"testing"

	"github.com/google/uuid"
)

func TestDefaultRolesStableCatalog(t *testing.T) {
	roles := DefaultRoles()
	if len(roles) == 0 {
		t.Fatal("DefaultRoles is empty")
	}
	seen := make(map[string]struct{})
	var hasSuperAdmin bool
	for _, role := range roles {
		if _, err := uuid.Parse(role.RoleID); err != nil {
			t.Fatalf("RoleID %q is not UUID: %v", role.RoleID, err)
		}
		if _, ok := seen[role.RoleID]; ok {
			t.Fatalf("duplicate role_id %s", role.RoleID)
		}
		seen[role.RoleID] = struct{}{}
		if role.RoleID == SuperAdminRoleID && role.System && role.Name != "" && role.Description != "" {
			hasSuperAdmin = true
		}
	}
	if !hasSuperAdmin {
		t.Fatal("super admin role missing or incomplete")
	}
}

func TestDefaultRolePermissionsReferenceCatalog(t *testing.T) {
	roles := make(map[string]struct{})
	for _, role := range DefaultRoles() {
		roles[role.RoleID] = struct{}{}
	}
	seen := make(map[string]struct{})
	for _, binding := range DefaultRolePermissions() {
		if _, ok := roles[binding.RoleID]; !ok {
			t.Fatalf("binding references unknown role_id %s", binding.RoleID)
		}
		if _, err := uuid.Parse(binding.PermissionID); err != nil {
			t.Fatalf("PermissionID %q is not UUID: %v", binding.PermissionID, err)
		}
		key := binding.RoleID + ":" + binding.PermissionID
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate role permission binding %s", key)
		}
		seen[key] = struct{}{}
	}
}
