package rbacbaseline

import (
	"testing"

	"github.com/google/uuid"
)

func TestDefaultRolesStableBaseline(t *testing.T) {
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

func TestDefaultPermissionsStableBaseline(t *testing.T) {
	permissions := DefaultPermissions()
	if len(permissions) == 0 {
		t.Fatal("DefaultPermissions is empty")
	}
	seenIDs := make(map[string]struct{})
	seenRoutes := make(map[string]struct{})
	var hasListUsers bool
	var hasCreateUser bool
	for _, permission := range permissions {
		if _, err := uuid.Parse(permission.PermissionID); err != nil {
			t.Fatalf("PermissionID %q is not UUID: %v", permission.PermissionID, err)
		}
		if _, ok := seenIDs[permission.PermissionID]; ok {
			t.Fatalf("duplicate permission_id %s", permission.PermissionID)
		}
		seenIDs[permission.PermissionID] = struct{}{}
		routeKey := permission.Method + " " + permission.PathTemplate
		if _, ok := seenRoutes[routeKey]; ok {
			t.Fatalf("duplicate route %s", routeKey)
		}
		seenRoutes[routeKey] = struct{}{}
		if permission.Method == "GET" && permission.PathTemplate == "/api/v1/users" {
			hasListUsers = true
		}
		if permission.Method == "POST" && permission.PathTemplate == "/api/v1/users" {
			hasCreateUser = true
		}
	}
	if !hasListUsers {
		t.Fatal("GET /api/v1/users permission missing")
	}
	if !hasCreateUser {
		t.Fatal("POST /api/v1/users permission missing")
	}
}

func TestDefaultPermissionsCoverAuthorizableRoutes(t *testing.T) {
	expected := map[string]struct{}{
		"GET /api/v1/users":                                        {},
		"POST /api/v1/users":                                       {},
		"GET /api/v1/users/:user_id":                               {},
		"GET /api/v1/permissions":                                  {},
		"POST /api/v1/permissions":                                 {},
		"GET /api/v1/permissions/route-diff":                       {},
		"GET /api/v1/permissions/users/:user_id/effective":         {},
		"GET /api/v1/permissions/:permission_id":                   {},
		"PUT /api/v1/permissions/:permission_id":                   {},
		"POST /api/v1/permissions/:permission_id/enable":           {},
		"POST /api/v1/permissions/:permission_id/disable":          {},
		"GET /api/v1/roles":                                        {},
		"POST /api/v1/roles":                                       {},
		"GET /api/v1/roles/:role_id":                               {},
		"PATCH /api/v1/roles/:role_id":                             {},
		"PATCH /api/v1/roles/:role_id/status":                      {},
		"GET /api/v1/users/:user_id/roles":                         {},
		"PUT /api/v1/users/:user_id/roles":                         {},
		"POST /api/v1/users/:user_id/roles":                        {},
		"DELETE /api/v1/users/:user_id/roles/:role_id":             {},
		"GET /api/v1/roles/:role_id/permissions":                   {},
		"PUT /api/v1/roles/:role_id/permissions":                   {},
		"POST /api/v1/roles/:role_id/permissions":                  {},
		"DELETE /api/v1/roles/:role_id/permissions/:permission_id": {},
	}
	actual := make(map[string]struct{}, len(DefaultPermissions()))
	for _, permission := range DefaultPermissions() {
		actual[permission.Method+" "+permission.PathTemplate] = struct{}{}
	}
	for route := range expected {
		if _, ok := actual[route]; !ok {
			t.Fatalf("permission catalog missing route %s", route)
		}
	}
	for route := range actual {
		if _, ok := expected[route]; !ok {
			t.Fatalf("permission catalog has unexpected route %s", route)
		}
	}
}

func TestDefaultRolePermissionsReferenceBaseline(t *testing.T) {
	roles := make(map[string]struct{})
	for _, role := range DefaultRoles() {
		roles[role.RoleID] = struct{}{}
	}
	permissions := make(map[string]struct{})
	for _, permission := range DefaultPermissions() {
		permissions[permission.PermissionID] = struct{}{}
	}
	seen := make(map[string]struct{})
	for _, binding := range DefaultRolePermissions() {
		if _, ok := roles[binding.RoleID]; !ok {
			t.Fatalf("binding references unknown role_id %s", binding.RoleID)
		}
		if _, ok := permissions[binding.PermissionID]; !ok {
			t.Fatalf("binding references unknown permission_id %s", binding.PermissionID)
		}
		key := binding.RoleID + ":" + binding.PermissionID
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate role permission binding %s", key)
		}
		seen[key] = struct{}{}
	}
	if len(seen) != len(permissions) {
		t.Fatalf("bindings = %d, want one super admin binding per permission %d", len(seen), len(permissions))
	}
	for permissionID := range permissions {
		key := SuperAdminRoleID + ":" + permissionID
		if _, ok := seen[key]; !ok {
			t.Fatalf("super admin binding missing permission_id %s", permissionID)
		}
	}
}
