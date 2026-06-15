package catalog

import (
	"testing"

	"github.com/google/uuid"
)

func TestDefaultPermissionsStableCatalog(t *testing.T) {
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
