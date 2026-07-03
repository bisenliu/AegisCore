package casbin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/ent"
	"github.com/aegiscore/user-service/ent/enttest"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestPolicyLoaderLoadsActiveBindings(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000102")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000103")
	role := createPolicyTestRole(t, client, roleID, true)
	permission := createPolicyTestPermission(t, client, permissionID, "GET", "/api/v1/users", true)
	createPolicyTestRolePermission(t, client, role.ID, permission.ID)

	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(ctx)
	require.NoError(t, err)
	assertHasRule(t, policies.PermissionRules, roleID, "/api/v1/users", "GET")
}

func TestPolicyLoaderSkipsInactiveRolesAndPermissions(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	activeRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")
	inactiveRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	activePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000205")
	inactivePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000206")
	activeRole := createPolicyTestRole(t, client, activeRoleID, true)
	inactiveRole := createPolicyTestRole(t, client, inactiveRoleID, false)
	activePermission := createPolicyTestPermission(t, client, activePermissionID, "GET", "/api/v1/active", true)
	inactivePermission := createPolicyTestPermission(t, client, inactivePermissionID, "POST", "/api/v1/inactive", false)
	createPolicyTestRolePermission(t, client, activeRole.ID, activePermission.ID)
	createPolicyTestRolePermission(t, client, activeRole.ID, inactivePermission.ID)
	createPolicyTestRolePermission(t, client, inactiveRole.ID, activePermission.ID)

	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(ctx)
	require.NoError(t, err)
	assertHasRule(t, policies.PermissionRules, activeRoleID, "/api/v1/active", "GET")
	assertMissingRule(t, policies.PermissionRules, activeRoleID, "/api/v1/inactive", "POST")
	assertMissingRule(t, policies.PermissionRules, inactiveRoleID, "/api/v1/active", "GET")
}

func TestPolicyLoaderAddsSuperAdminWildcard(t *testing.T) {
	client := newPolicyTestClient(t)
	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(context.Background())
	require.NoError(t, err)
	assertHasRule(t, policies.PermissionRules, uuid.MustParse(rbacbaseline.SuperAdminRoleID), policyWildcard, policyWildcard)
}

func TestPolicyLoaderUsesRoleIDSubjectWithoutRoleCode(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000208")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000209")
	role := createPolicyTestRole(t, client, roleID, true)
	permission := createPolicyTestPermission(t, client, permissionID, "PATCH", "/api/v1/roles/:role_id", true)
	createPolicyTestRolePermission(t, client, role.ID, permission.ID)

	loader := NewPolicyLoader(LoaderParams{Client: client})
	policies, err := loader.LoadPolicies(ctx)
	require.NoError(t, err)
	assertHasRule(t, policies.PermissionRules, roleID, "/api/v1/roles/:role_id", "PATCH")
}

func TestUserRoleResolverCachesAndInvalidatesActiveRoles(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
	activeRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")
	laterRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000303")
	inactiveRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000304")
	user := createPolicyTestUser(t, client, userID, "resolver@example.com")
	activeRole := createPolicyTestRole(t, client, activeRoleID, true)
	laterRole := createPolicyTestRole(t, client, laterRoleID, true)
	inactiveRole := createPolicyTestRole(t, client, inactiveRoleID, false)
	createPolicyTestUserRole(t, client, user.ID, activeRole.ID)
	createPolicyTestUserRole(t, client, user.ID, inactiveRole.ID)
	resolver := newTestUserRoleResolver(t, client, time.Minute)

	first, err := resolver.RolesForUser(ctx, userID)
	require.NoError(t, err)
	assertRoleIDs(t, first, []uuid.UUID{activeRoleID})

	createPolicyTestUserRole(t, client, user.ID, laterRole.ID)
	cached, err := resolver.RolesForUser(ctx, userID)
	require.NoError(t, err)
	assertRoleIDs(t, cached, []uuid.UUID{activeRoleID})

	resolver.InvalidateUserRole(userID)
	reloaded, err := resolver.RolesForUser(ctx, userID)
	require.NoError(t, err)
	assertRoleIDs(t, reloaded, []uuid.UUID{activeRoleID, laterRoleID})
}

func TestUserRoleResolverCoalescesConcurrentMisses(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000402")
	user := createPolicyTestUser(t, client, userID, "resolver-concurrent@example.com")
	role := createPolicyTestRole(t, client, roleID, true)
	createPolicyTestUserRole(t, client, user.ID, role.ID)

	var roleQueries atomic.Int64
	queryStarted := make(chan struct{})
	releaseQuery := make(chan struct{})
	client.Role.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, q ent.Query) (ent.Value, error) {
			if roleQueries.Add(1) == 1 {
				close(queryStarted)
				<-releaseQuery
			}
			return next.Query(ctx, q)
		})
	}))

	resolver := newTestUserRoleResolver(t, client, time.Minute)
	const calls = 8
	var wg sync.WaitGroup
	wg.Add(calls)
	errCh := make(chan error, calls)
	for range calls {
		go func() {
			defer wg.Done()
			roleIDs, err := resolver.RolesForUser(ctx, userID)
			if err != nil {
				errCh <- err
				return
			}
			if len(roleIDs) != 1 || roleIDs[0] != roleID {
				errCh <- errors.New("unexpected role ids")
			}
		}()
	}

	<-queryStarted
	close(releaseQuery)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), roleQueries.Load())
}

func TestNewUserRoleResolverRequiresConfigInstance(t *testing.T) {
	_, err := NewUserRoleResolver(UserRoleResolverParams{
		Config: &config.Config{LocalCache: config.LocalCacheConfig{}},
		Client: newPolicyTestClient(t),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "local_cache.rbac_user_roles is required")
}

func newPolicyTestClient(t *testing.T) *ent.Client {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:casbin_policy_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newTestUserRoleResolver(t *testing.T, client *ent.Client, ttl time.Duration) *entUserRoleResolver {
	t.Helper()
	resolver := &entUserRoleResolver{client: client}
	cache, err := localcache.New[uuid.UUID, []uuid.UUID](localcache.Config[uuid.UUID]{
		Name:        "rbac_user_roles_test",
		Capacity:    100,
		TTL:         ttl,
		LoadTimeout: time.Second,
		KeyString:   func(userID uuid.UUID) string { return userID.String() },
	}, func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return resolver.loadRolesForUser(ctx, userID)
	}, cloneRoleIDs)
	require.NoError(t, err)
	resolver.cache = cache
	t.Cleanup(cache.Close)
	return resolver
}

func createPolicyTestUser(t *testing.T, client *ent.Client, userID uuid.UUID, username string) *ent.User {
	t.Helper()
	user, err := client.User.Create().SetUserID(userID).SetNickname(username).SetUsername(username).SetPasswordHash("hash").Save(context.Background())
	require.NoError(t, err)
	return user
}

func createPolicyTestRole(t *testing.T, client *ent.Client, roleID uuid.UUID, active bool) *ent.Role {
	t.Helper()
	role, err := client.Role.Create().SetRoleID(roleID).SetName(roleID.String()).SetActive(active).Save(context.Background())
	require.NoError(t, err)
	return role
}

func createPolicyTestPermission(t *testing.T, client *ent.Client, permissionID uuid.UUID, method string, path string, active bool) *ent.Permission {
	t.Helper()
	permission, err := client.Permission.Create().SetPermissionID(permissionID).SetName(permissionID.String()).SetModule("test").SetHTTPMethod(method).SetPathTemplate(path).SetActive(active).Save(context.Background())
	require.NoError(t, err)
	return permission
}

func createPolicyTestUserRole(t *testing.T, client *ent.Client, userID int64, roleID int64) {
	t.Helper()
	_, err := client.UserRole.Create().SetUserID(userID).SetRoleID(roleID).Save(context.Background())
	require.NoError(t, err)
}

func createPolicyTestRolePermission(t *testing.T, client *ent.Client, roleID int64, permissionID int64) {
	t.Helper()
	_, err := client.RolePermission.Create().SetRoleID(roleID).SetPermissionID(permissionID).Save(context.Background())
	require.NoError(t, err)
}

func assertHasRule(t *testing.T, rules []PermissionRule, roleID uuid.UUID, path string, method string) {
	t.Helper()
	require.Contains(t, rules, PermissionRule{RoleID: roleID, PathTemplate: path, HTTPMethod: method})
}

func assertMissingRule(t *testing.T, rules []PermissionRule, roleID uuid.UUID, path string, method string) {
	t.Helper()
	for _, rule := range rules {
		require.NotEqual(t, PermissionRule{RoleID: roleID, PathTemplate: path, HTTPMethod: method}, rule)
	}
}

func assertRoleIDs(t *testing.T, got []uuid.UUID, want []uuid.UUID) {
	t.Helper()
	require.Equal(t, want, got)
}
