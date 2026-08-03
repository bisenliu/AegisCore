package casbin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/testing/containers"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/persistence/ent/enttest"
	entuserrole "github.com/aegiscore/user-service/internal/persistence/ent/userrole"
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

	loader := NewPolicyLoader(client)
	policies, err := loader.LoadPoliciesAtLeast(ctx, 0)
	require.NoError(t, err)
	require.Zero(t, policies.Revision)
	assertHasRule(t, policies.PermissionRules, roleID, "/api/v1/users", "GET")
}

func TestPolicyLoaderSkipsInactiveRoles(t *testing.T) {
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

	loader := NewPolicyLoader(client)
	policies, err := loader.LoadPoliciesAtLeast(ctx, 0)
	require.NoError(t, err)
	assertHasRule(t, policies.PermissionRules, activeRoleID, "/api/v1/active", "GET")
	assertHasRule(t, policies.PermissionRules, activeRoleID, "/api/v1/inactive", "POST")
	assertMissingRule(t, policies.PermissionRules, inactiveRoleID, "/api/v1/active", "GET")
}

func TestPolicyLoaderAddsSuperAdminWildcard(t *testing.T) {
	client := newPolicyTestClient(t)
	loader := NewPolicyLoader(client)
	policies, err := loader.LoadPoliciesAtLeast(context.Background(), 0)
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

	loader := NewPolicyLoader(client)
	policies, err := loader.LoadPoliciesAtLeast(ctx, 0)
	require.NoError(t, err)
	assertHasRule(t, policies.PermissionRules, roleID, "/api/v1/roles/:role_id", "PATCH")
}

func TestPolicyLoaderWaitForTargetRespectsContext(t *testing.T) {
	loader := NewPolicyLoader(newPolicyTestClient(t))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, err := loader.LoadPoliciesAtLeast(ctx, 1)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPolicyLoaderRejectsNegativeTarget(t *testing.T) {
	loader := NewPolicyLoader(newPolicyTestClient(t))

	_, err := loader.LoadPoliciesAtLeast(context.Background(), -1)

	require.ErrorContains(t, err, "target revision must not be negative")
}

func TestPolicyLoaderReturnsRevisionQueryError(t *testing.T) {
	client := newPolicyTestClient(t)
	wantErr := errors.New("revision query failed")
	client.RbacPolicyRevision.Intercept(ent.InterceptFunc(func(ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(context.Context, ent.Query) (ent.Value, error) {
			return nil, wantErr
		})
	}))

	_, err := NewPolicyLoader(client).LoadPoliciesAtLeast(context.Background(), 0)

	require.ErrorIs(t, err, wantErr)
	require.ErrorContains(t, err, "load latest casbin policy revision")
}

func TestPolicyLoaderReturnsRuleQueryError(t *testing.T) {
	client := newPolicyTestClient(t)
	wantErr := errors.New("rule query failed")
	client.RolePermission.Intercept(ent.InterceptFunc(func(ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(context.Context, ent.Query) (ent.Value, error) {
			return nil, wantErr
		})
	}))

	_, err := NewPolicyLoader(client).LoadPoliciesAtLeast(context.Background(), 0)

	require.ErrorIs(t, err, wantErr)
	require.ErrorContains(t, err, "load casbin permission policies")
}

func TestPolicyLoaderReturnsBeginTransactionError(t *testing.T) {
	client := newPolicyTestClient(t)
	require.NoError(t, client.Close())

	_, err := NewPolicyLoader(client).LoadPoliciesAtLeast(context.Background(), 0)

	require.ErrorContains(t, err, "begin casbin policy snapshot")
}

func TestPolicyLoaderReturnsRollbackError(t *testing.T) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:casbin_policy_rollback_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	require.NoError(t, err)
	baseDriver := entsql.OpenDB(dialect.SQLite, db)
	schemaClient := ent.NewClient(ent.Driver(baseDriver))
	require.NoError(t, schemaClient.Schema.Create(context.Background()))
	wantErr := errors.New("rollback failed")
	client := ent.NewClient(ent.Driver(&policyRollbackErrorDriver{Driver: baseDriver, err: wantErr}))
	t.Cleanup(func() { _ = client.Close() })

	_, err = NewPolicyLoader(client).LoadPoliciesAtLeast(context.Background(), 0)

	require.ErrorIs(t, err, wantErr)
	require.ErrorContains(t, err, "rollback casbin policy snapshot")
}

func TestPolicyLoaderPostgresBindsRulesToRevisionAndRetriesFreshSnapshot(t *testing.T) {
	client, driver := newPostgresPolicyTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000702")
	role := createPolicyTestRole(t, client, roleID, true)
	permission := createPolicyTestPermission(t, client, permissionID, "DELETE", "/api/v1/users/:id", true)

	writer, err := client.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = writer.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(permission.ID).Save(ctx)
	require.NoError(t, err)
	_, err = writer.RbacPolicyRevision.Create().SetID(1).SetReason("role_permission_bound").SetCreatedAt(time.Now().UnixMilli()).Save(ctx)
	require.NoError(t, err)
	beginCountBeforeLoad := driver.begins.Load()

	resultCh := make(chan PolicySet, 1)
	errCh := make(chan error, 1)
	go func() {
		policies, loadErr := NewPolicyLoader(client).LoadPoliciesAtLeast(ctx, 1)
		if loadErr != nil {
			errCh <- loadErr
			return
		}
		resultCh <- policies
	}()
	require.Eventually(t, func() bool {
		return driver.begins.Load() >= beginCountBeforeLoad+2
	}, time.Second, 5*time.Millisecond)
	require.Equal(t, int32(sql.LevelRepeatableRead), driver.isolation.Load())
	require.True(t, driver.readOnly.Load())
	select {
	case result := <-resultCh:
		require.Failf(t, "loader returned stale snapshot", "unexpected policy set: %+v", result)
	case loadErr := <-errCh:
		require.NoError(t, loadErr)
	default:
	}
	require.NoError(t, writer.Commit())

	select {
	case policies := <-resultCh:
		require.Equal(t, int64(1), policies.Revision)
		assertHasRule(t, policies.PermissionRules, roleID, "/api/v1/users/:id", "DELETE")
	case loadErr := <-errCh:
		require.NoError(t, loadErr)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}
	// 至少两个 loader transaction 证明低 revision snapshot 已关闭并重新打开；
	// 第二个 transaction 的首条查询可能在 writer commit 之后才建立 PostgreSQL snapshot。
	require.GreaterOrEqual(t, driver.begins.Load(), beginCountBeforeLoad+2)
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
	first[0] = laterRoleID

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

func TestUserRoleResolverSuppressesStaleSingleUserLoadAfterInvalidation(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	firstRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000502")
	secondRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000503")
	user := createPolicyTestUser(t, client, userID, "resolver-stale-user@example.com")
	firstRole := createPolicyTestRole(t, client, firstRoleID, true)
	secondRole := createPolicyTestRole(t, client, secondRoleID, true)
	createPolicyTestUserRole(t, client, user.ID, firstRole.ID)

	queryStarted := make(chan struct{})
	releaseQuery := make(chan struct{})
	var roleQueries atomic.Int64
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
	staleErr := make(chan error, 1)
	go func() {
		_, err := resolver.RolesForUser(ctx, userID)
		staleErr <- err
	}()
	<-queryStarted
	createPolicyTestUserRole(t, client, user.ID, secondRole.ID)
	resolver.InvalidateUserRole(userID)
	close(releaseQuery)
	require.ErrorIs(t, <-staleErr, errUserRoleCacheGenerationStale)

	reloaded, err := resolver.RolesForUser(ctx, userID)
	require.NoError(t, err)
	assertRoleIDs(t, reloaded, []uuid.UUID{firstRoleID, secondRoleID})
	require.Equal(t, int64(2), roleQueries.Load())
}

func TestUserRoleResolverSuppressesStaleLoadsAfterInvalidateAll(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	firstUserID := uuid.MustParse("018f0000-0000-7000-8000-000000000511")
	secondUserID := uuid.MustParse("018f0000-0000-7000-8000-000000000512")
	firstRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000513")
	secondRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000514")
	firstUser := createPolicyTestUser(t, client, firstUserID, "resolver-stale-all-1@example.com")
	secondUser := createPolicyTestUser(t, client, secondUserID, "resolver-stale-all-2@example.com")
	firstRole := createPolicyTestRole(t, client, firstRoleID, true)
	secondRole := createPolicyTestRole(t, client, secondRoleID, true)
	createPolicyTestUserRole(t, client, firstUser.ID, firstRole.ID)
	createPolicyTestUserRole(t, client, secondUser.ID, secondRole.ID)

	queriesStarted := make(chan struct{})
	releaseQueries := make(chan struct{})
	var roleQueries atomic.Int64
	client.Role.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, q ent.Query) (ent.Value, error) {
			queryNumber := roleQueries.Add(1)
			if queryNumber <= 2 {
				if queryNumber == 2 {
					close(queriesStarted)
				}
				<-releaseQueries
			}
			return next.Query(ctx, q)
		})
	}))

	resolver := newTestUserRoleResolver(t, client, time.Minute)
	errCh := make(chan error, 2)
	go func() {
		_, err := resolver.RolesForUser(ctx, firstUserID)
		errCh <- err
	}()
	go func() {
		_, err := resolver.RolesForUser(ctx, secondUserID)
		errCh <- err
	}()
	<-queriesStarted
	resolver.InvalidateAllUserRoles()
	close(releaseQueries)
	require.ErrorIs(t, <-errCh, errUserRoleCacheGenerationStale)
	require.ErrorIs(t, <-errCh, errUserRoleCacheGenerationStale)

	firstReloaded, err := resolver.RolesForUser(ctx, firstUserID)
	require.NoError(t, err)
	assertRoleIDs(t, firstReloaded, []uuid.UUID{firstRoleID})
	secondReloaded, err := resolver.RolesForUser(ctx, secondUserID)
	require.NoError(t, err)
	assertRoleIDs(t, secondReloaded, []uuid.UUID{secondRoleID})
	require.Equal(t, int64(4), roleQueries.Load())
}

func TestUserRoleResolverStaleLoadFailsClosedAndReloadsFinalState(t *testing.T) {
	client := newPolicyTestClient(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000521")
	oldRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000522")
	finalRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000523")
	user := createPolicyTestUser(t, client, userID, "resolver-stale-fail-closed@example.com")
	oldRole := createPolicyTestRole(t, client, oldRoleID, true)
	finalRole := createPolicyTestRole(t, client, finalRoleID, true)
	createPolicyTestUserRole(t, client, user.ID, oldRole.ID)

	queryStarted := make(chan struct{})
	releaseQuery := make(chan struct{})
	var roleQueries atomic.Int64
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
	staleErr := make(chan error, 1)
	go func() {
		_, err := resolver.RolesForUser(ctx, userID)
		staleErr <- err
	}()
	<-queryStarted
	resolver.InvalidateUserRole(userID)
	close(releaseQuery)
	require.ErrorIs(t, <-staleErr, errUserRoleCacheGenerationStale)

	_, err := client.UserRole.Delete().Where(entuserrole.UserIDEQ(user.ID)).Exec(ctx)
	require.NoError(t, err)
	createPolicyTestUserRole(t, client, user.ID, finalRole.ID)
	reloaded, err := resolver.RolesForUser(ctx, userID)
	require.NoError(t, err)
	assertRoleIDs(t, reloaded, []uuid.UUID{finalRoleID})
	require.Equal(t, int64(2), roleQueries.Load())
}

func TestNewUserRoleResolverUsesRBACFeatureConfig(t *testing.T) {
	result, err := NewUserRoleResolver(UserRoleResolverParams{
		Settings: serviceconfig.RBACSettings{UserRoleCache: serviceconfig.FeatureCacheConfig{
			Enabled: true, Size: 321, TTL: time.Minute, LoadTimeout: time.Second,
		}},
		Client: newPolicyTestClient(t),
	})
	require.NoError(t, err)
	require.EqualValues(t, 321, result.Stats.Stats().Capacity)
	require.NoError(t, result.Closer.Close())
	require.NoError(t, result.Closer.Close())
	_, err = result.Resolver.RolesForUser(context.Background(), uuid.New())
	require.ErrorIs(t, err, localcache.ErrClosed)
}

func TestNewUserRoleResolverRejectsNegativeCapacity(t *testing.T) {
	_, err := NewUserRoleResolver(UserRoleResolverParams{
		Settings: serviceconfig.RBACSettings{UserRoleCache: serviceconfig.FeatureCacheConfig{
			Enabled: true, Size: -1, TTL: time.Minute, LoadTimeout: time.Second,
		}},
		Client: newPolicyTestClient(t),
	})
	require.ErrorIs(t, err, localcache.ErrCapacityRequired)
}

func TestDisabledUserRoleResolverReadsThroughAndInvalidationIsSafe(t *testing.T) {
	client := newPolicyTestClient(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000601")
	firstRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000602")
	secondRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000603")
	user := createPolicyTestUser(t, client, userID, "resolver-disabled@example.com")
	firstRole := createPolicyTestRole(t, client, firstRoleID, true)
	secondRole := createPolicyTestRole(t, client, secondRoleID, true)
	createPolicyTestUserRole(t, client, user.ID, firstRole.ID)

	result, err := NewUserRoleResolver(UserRoleResolverParams{
		Settings: serviceconfig.RBACSettings{UserRoleCache: serviceconfig.FeatureCacheConfig{
			Enabled: false, Size: -1, TTL: -time.Second, LoadTimeout: -time.Second,
		}},
		Client: client,
	})
	require.NoError(t, err)

	first, err := result.Resolver.RolesForUser(context.Background(), userID)
	require.NoError(t, err)
	assertRoleIDs(t, first, []uuid.UUID{firstRoleID})
	first[0] = secondRoleID
	createPolicyTestUserRole(t, client, user.ID, secondRole.ID)
	second, err := result.Resolver.RolesForUser(context.Background(), userID)
	require.NoError(t, err)
	assertRoleIDs(t, second, []uuid.UUID{firstRoleID, secondRoleID})

	result.Resolver.InvalidateUserRole(userID)
	result.Resolver.InvalidateAllUserRoles()
	require.NoError(t, result.Closer.Close())
	require.NoError(t, result.Closer.Close())
	require.Equal(t, rbacUserRolesCacheName, result.Stats.Name())
	require.EqualValues(t, 2, result.Stats.Stats().LoadSuccess)
	require.Zero(t, result.Stats.Stats().Capacity)
}

func newPolicyTestClient(t *testing.T) *ent.Client {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:casbin_policy_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newPostgresPolicyTestClient(t *testing.T) (*ent.Client, *policyCountingDriver) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	db, err := sql.Open("pgx", postgres.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm")
	require.NoError(t, err)
	driver := &policyCountingDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}
	client := ent.NewClient(ent.Driver(driver))
	require.NoError(t, client.Schema.Create(ctx))
	t.Cleanup(func() { _ = client.Close() })
	return client, driver
}

type policyCountingDriver struct {
	dialect.Driver
	begins    atomic.Int64
	isolation atomic.Int32
	readOnly  atomic.Bool
}

func (d *policyCountingDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	if opts != nil {
		d.isolation.Store(int32(opts.Isolation))
		d.readOnly.Store(opts.ReadOnly)
	}
	d.begins.Add(1)
	return d.Driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	}).BeginTx(ctx, opts)
}

type policyRollbackErrorDriver struct {
	dialect.Driver
	err error
}

func (d *policyRollbackErrorDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	tx, err := d.Driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	}).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &policyRollbackErrorTx{Tx: tx, err: d.err}, nil
}

type policyRollbackErrorTx struct {
	dialect.Tx
	err error
}

func (tx *policyRollbackErrorTx) Rollback() error {
	_ = tx.Tx.Rollback()
	return tx.err
}

func newTestUserRoleResolver(t *testing.T, client *ent.Client, ttl time.Duration) *entUserRoleResolver {
	t.Helper()
	resolver := &entUserRoleResolver{client: client}
	cache, err := localcache.NewLoadingCache[uuid.UUID, []uuid.UUID](localcache.Config{
		Name:        "rbac_user_roles_test",
		Capacity:    100,
		TTL:         ttl,
		LoadTimeout: time.Second,
	}, func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return resolver.loadCacheableRolesForUser(ctx, userID)
	})
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

func createPolicyTestPermission(t *testing.T, client *ent.Client, permissionID uuid.UUID, method string, path string, _ bool) *ent.Permission {
	t.Helper()
	permission, err := client.Permission.Create().SetPermissionID(permissionID).SetName(permissionID.String()).SetModule("test").SetHTTPMethod(method).SetPathTemplate(path).Save(context.Background())
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
