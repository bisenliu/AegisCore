package casbin

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/aegiscore/user-service/ent"
	"github.com/aegiscore/user-service/ent/enttest"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestPolicyLoaderLoadsActiveBindings(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000102")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000103")
	user := createPolicyTestUser(t, client, userID, "active@example.com")
	role := createPolicyTestRole(t, client, roleID, true)
	permission := createPolicyTestPermission(t, client, permissionID, "GET", "/api/v1/users", true)
	createPolicyTestUserRole(t, client, user.ID, role.ID)
	createPolicyTestRolePermission(t, client, role.ID, permission.ID)

	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(ctx)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}
	assertHasGroup(t, policies.GroupingPolicies, userID, roleID)
	assertHasRule(t, policies.PermissionRules, roleID, "/api/v1/users", "GET")
}

func TestPolicyLoaderSkipsInactiveRolesAndPermissions(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	activeUserID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")
	inactiveRoleUserID := uuid.MustParse("018f0000-0000-7000-8000-000000000202")
	activeRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")
	inactiveRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	activePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000205")
	inactivePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000206")
	activeUser := createPolicyTestUser(t, client, activeUserID, "active-filter@example.com")
	inactiveRoleUser := createPolicyTestUser(t, client, inactiveRoleUserID, "inactive-role@example.com")
	activeRole := createPolicyTestRole(t, client, activeRoleID, true)
	inactiveRole := createPolicyTestRole(t, client, inactiveRoleID, false)
	activePermission := createPolicyTestPermission(t, client, activePermissionID, "GET", "/api/v1/active", true)
	inactivePermission := createPolicyTestPermission(t, client, inactivePermissionID, "POST", "/api/v1/inactive", false)
	createPolicyTestUserRole(t, client, activeUser.ID, activeRole.ID)
	createPolicyTestUserRole(t, client, inactiveRoleUser.ID, inactiveRole.ID)
	createPolicyTestRolePermission(t, client, activeRole.ID, activePermission.ID)
	createPolicyTestRolePermission(t, client, activeRole.ID, inactivePermission.ID)
	createPolicyTestRolePermission(t, client, inactiveRole.ID, activePermission.ID)

	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(ctx)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}
	assertHasGroup(t, policies.GroupingPolicies, activeUserID, activeRoleID)
	assertMissingGroup(t, policies.GroupingPolicies, inactiveRoleUserID, inactiveRoleID)
	assertHasRule(t, policies.PermissionRules, activeRoleID, "/api/v1/active", "GET")
	assertMissingRule(t, policies.PermissionRules, activeRoleID, "/api/v1/inactive", "POST")
	assertMissingRule(t, policies.PermissionRules, inactiveRoleID, "/api/v1/active", "GET")
}

func TestPolicyLoaderAddsSuperAdminWildcard(t *testing.T) {
	client := newPolicyTestClient(t)
	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(context.Background())
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}
	assertHasRule(t, policies.PermissionRules, uuid.MustParse(rbacbaseline.SuperAdminRoleID), policyWildcard, policyWildcard)
}

func TestPolicyLoaderUsesRoleIDSubjectWithoutRoleCode(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000207")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000208")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000209")
	user := createPolicyTestUser(t, client, userID, "role-id-subject@example.com")
	role := createPolicyTestRole(t, client, roleID, true)
	permission := createPolicyTestPermission(t, client, permissionID, "PATCH", "/api/v1/roles/:role_id", true)
	createPolicyTestUserRole(t, client, user.ID, role.ID)
	createPolicyTestRolePermission(t, client, role.ID, permission.ID)

	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(ctx)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}
	assertHasGroup(t, policies.GroupingPolicies, userID, roleID)
	assertHasRule(t, policies.PermissionRules, roleID, "/api/v1/roles/:role_id", "PATCH")
}

func newPolicyTestClient(t *testing.T) *ent.Client {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:casbin_policy_test_%s?mode=memory&cache=shared&_fk=1", uuid.NewString()))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func createPolicyTestUser(t *testing.T, client *ent.Client, userID uuid.UUID, username string) *ent.User {
	t.Helper()
	user, err := client.User.Create().SetUserID(userID).SetNickname(username).SetUsername(username).SetPasswordHash("hash").Save(context.Background())
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func createPolicyTestRole(t *testing.T, client *ent.Client, roleID uuid.UUID, active bool) *ent.Role {
	t.Helper()
	role, err := client.Role.Create().SetRoleID(roleID).SetName(roleID.String()).SetActive(active).Save(context.Background())
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	return role
}

func createPolicyTestPermission(t *testing.T, client *ent.Client, permissionID uuid.UUID, method string, path string, active bool) *ent.Permission {
	t.Helper()
	permission, err := client.Permission.Create().SetPermissionID(permissionID).SetName(permissionID.String()).SetModule("test").SetHTTPMethod(method).SetPathTemplate(path).SetActive(active).Save(context.Background())
	if err != nil {
		t.Fatalf("create permission: %v", err)
	}
	return permission
}

func createPolicyTestUserRole(t *testing.T, client *ent.Client, userID int64, roleID int64) {
	t.Helper()
	if _, err := client.UserRole.Create().SetUserID(userID).SetRoleID(roleID).Save(context.Background()); err != nil {
		t.Fatalf("create user role: %v", err)
	}
}

func createPolicyTestRolePermission(t *testing.T, client *ent.Client, roleID int64, permissionID int64) {
	t.Helper()
	if _, err := client.RolePermission.Create().SetRoleID(roleID).SetPermissionID(permissionID).Save(context.Background()); err != nil {
		t.Fatalf("create role permission: %v", err)
	}
}

func assertHasGroup(t *testing.T, groups []GroupingPolicy, userID uuid.UUID, roleID uuid.UUID) {
	t.Helper()
	for _, group := range groups {
		if group.UserID == userID && group.RoleID == roleID {
			return
		}
	}
	t.Fatalf("missing group user=%s role=%s in %#v", userID, roleID, groups)
}

func assertMissingGroup(t *testing.T, groups []GroupingPolicy, userID uuid.UUID, roleID uuid.UUID) {
	t.Helper()
	for _, group := range groups {
		if group.UserID == userID && group.RoleID == roleID {
			t.Fatalf("unexpected group user=%s role=%s", userID, roleID)
		}
	}
}

func assertHasRule(t *testing.T, rules []PermissionRule, roleID uuid.UUID, path string, method string) {
	t.Helper()
	for _, rule := range rules {
		if rule.RoleID == roleID && rule.PathTemplate == path && rule.HTTPMethod == method {
			return
		}
	}
	t.Fatalf("missing rule role=%s path=%s method=%s in %#v", roleID, path, method, rules)
}

func assertMissingRule(t *testing.T, rules []PermissionRule, roleID uuid.UUID, path string, method string) {
	t.Helper()
	for _, rule := range rules {
		if rule.RoleID == roleID && rule.PathTemplate == path && rule.HTTPMethod == method {
			t.Fatalf("unexpected rule role=%s path=%s method=%s", roleID, path, method)
		}
	}
}
