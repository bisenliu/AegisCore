package rbacbaseline

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDefaultRolesStableBaseline(t *testing.T) {
	roles := DefaultRoles()
	require.NotEmpty(t, roles)

	seen := make(map[string]struct{})
	var hasSuperAdmin bool
	for _, role := range roles {
		_, err := uuid.Parse(role.RoleID)
		require.NoError(t, err, "RoleID %q must be UUID", role.RoleID)
		require.NotContains(t, seen, role.RoleID)
		seen[role.RoleID] = struct{}{}
		if role.RoleID == SuperAdminRoleID && role.System && role.Name != "" && role.Description != "" {
			hasSuperAdmin = true
		}
	}
	require.True(t, hasSuperAdmin, "super admin role missing or incomplete")
}

func TestDefaultPermissionsStableBaseline(t *testing.T) {
	permissions := DefaultPermissions()
	require.NotEmpty(t, permissions)

	seenIDs := make(map[string]struct{})
	seenRoutes := make(map[string]struct{})
	var hasListUsers bool
	var hasCreateUser bool
	for _, permission := range permissions {
		_, err := uuid.Parse(permission.PermissionID)
		require.NoError(t, err, "PermissionID %q must be UUID", permission.PermissionID)
		require.NotContains(t, seenIDs, permission.PermissionID)
		seenIDs[permission.PermissionID] = struct{}{}
		routeKey := permission.Method + " " + permission.PathTemplate
		require.NotContains(t, seenRoutes, routeKey)
		seenRoutes[routeKey] = struct{}{}
		if permission.Method == "GET" && permission.PathTemplate == "/api/v1/users" {
			hasListUsers = true
		}
		if permission.Method == "POST" && permission.PathTemplate == "/api/v1/users" {
			hasCreateUser = true
		}
	}
	require.True(t, hasListUsers, "GET /api/v1/users permission missing")
	require.True(t, hasCreateUser, "POST /api/v1/users permission missing")
}

func TestDefaultPermissionsCoverAuthorizableRoutes(t *testing.T) {
	expected := map[string]struct{}{
		"GET /api/v1/users":                                        {},
		"POST /api/v1/users":                                       {},
		"GET /api/v1/users/:user_id":                               {},
		"GET /api/v1/permissions":                                  {},
		"GET /api/v1/permissions/users/:user_id/effective":         {},
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
		require.Contains(t, actual, route, "permission catalog missing route")
	}
	for route := range actual {
		require.Contains(t, expected, route, "permission catalog has unexpected route")
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
		require.Contains(t, roles, binding.RoleID, "binding references unknown role_id")
		require.Contains(t, permissions, binding.PermissionID, "binding references unknown permission_id")
		key := binding.RoleID + ":" + binding.PermissionID
		require.NotContains(t, seen, key, "duplicate role permission binding")
		seen[key] = struct{}{}
	}
	require.Len(t, seen, len(permissions), "want one super admin binding per permission")
	for permissionID := range permissions {
		key := SuperAdminRoleID + ":" + permissionID
		require.Contains(t, seen, key, "super admin binding missing permission_id")
	}
}
