package e2e

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/testing/containers"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrbacoutbox "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyoutboxevent"
)

func TestRBACOutboxRedisRecoveryConvergesAllProjectionsWithoutNewWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	client := newRBACSyncPostgresClient(ctx, t)
	_, err := client.RbacPolicyRevisionCounter.Create().SetID(1).SetLastRevision(0).Save(ctx)
	require.NoError(t, err)

	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000024001")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000024002")
	resolver := staticRBACUserRoleResolver{roleID: roleID}
	engines := []*permissioncasbin.Engine{
		permissioncasbin.NewEngine(permissioncasbin.NewPolicyLoader(client), permissioncasbin.NopReloadMetrics(), resolver),
		permissioncasbin.NewEngine(permissioncasbin.NewPolicyLoader(client), permissioncasbin.NopReloadMetrics(), resolver),
	}
	for _, engine := range engines {
		applied, reloadErr := engine.ReloadToRevision(ctx, 0)
		require.NoError(t, reloadErr)
		require.Zero(t, applied)
		allowed, enforceErr := engine.Enforce(ctx, userID, "/api/v1/recovery", "GET")
		require.NoError(t, enforceErr)
		require.False(t, allowed)
	}

	roleWrite, err := rolepostgres.NewRoleStore(client).Create(ctx, roleapplication.CreateRoleInput{
		RoleID: roleID,
		Name:   "Redis Recovery Role",
		Active: true,
	}, roleapplication.PolicyChange{
		Kind:   roleapplication.PolicyChangeKindPolicyChanged,
		Reason: "role_created",
		RoleID: roleID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), roleWrite.Revision)
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000024003")
	permission, err := client.Permission.Create().
		SetPermissionID(permissionID).
		SetName("Recovery Read").
		SetModule("recovery").
		SetHTTPMethod("GET").
		SetPathTemplate("/api/v1/recovery").
		Save(ctx)
	require.NoError(t, err)
	bindingWrite, err := rolepostgres.NewRolePermissionStore(client).Add(ctx, roleID, roleapplication.PermissionReference{
		ID:           permission.ID,
		PermissionID: permission.PermissionID,
		Name:         permission.Name,
		Module:       permission.Module,
		HTTPMethod:   permission.HTTPMethod,
		PathTemplate: permission.PathTemplate,
		CreatedAt:    permission.CreatedAt,
		UpdatedAt:    permission.UpdatedAt,
	}, roleapplication.PolicyChange{
		Kind:         roleapplication.PolicyChangeKindPolicyChanged,
		Reason:       "role_permission_added",
		RoleID:       roleID,
		PermissionID: permissionID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), bindingWrite.Revision)
	finalRevision := bindingWrite.Revision

	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{
		Addr:         redisServer.Addr(),
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = redisClient.Close() })
	redisStore, err := permissionredis.NewStore(redisClient, "aegiscore-user-service-test", nil)
	require.NoError(t, err)
	dispatcher, err := permissionapplication.NewDispatcher(
		permissionpostgres.NewOutboxStore(client),
		redisStore,
		permissionapplication.DispatcherSettings{
			PollInterval:   time.Second,
			BatchSize:      10,
			ClaimTimeout:   time.Second,
			BackoffInitial: 10 * time.Millisecond,
			BackoffMax:     100 * time.Millisecond,
		},
		nil,
		nil,
		permissionapplication.NopMetrics(),
	)
	require.NoError(t, err)

	redisServer.Close()
	// 先制造一次投递失败，确认事件保留在 outbox；恢复 Redis 后不得依赖新的业务写入触发重试。
	require.Error(t, dispatcher.DispatchOnce(ctx))
	failed, err := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.RevisionEQ(finalRevision)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, permissionapplication.OutboxStatusFailed, failed.Status)
	require.Equal(t, 1, failed.AttemptCount)
	for _, engine := range engines {
		require.Zero(t, engine.AppliedRevision())
	}

	require.NoError(t, redisServer.Restart())
	// DispatchOnce 同时承担失败事件的到期重试，最终应把此前的全部 revision 推进到 Redis 权威版本。
	require.Eventually(t, func() bool {
		_ = dispatcher.DispatchOnce(ctx)
		delivered, queryErr := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.StatusEQ(permissionapplication.OutboxStatusDelivered)).Count(ctx)
		return queryErr == nil && delivered == 2
	}, 5*time.Second, 20*time.Millisecond)
	version, err := redisStore.CurrentVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, finalRevision, version)

	revisionSource := permissionpostgres.NewPolicyRevisionSource(client)
	for _, engine := range engines {
		// 两个独立 engine 都从同一数据库 revision 校准，验证恢复过程不是单实例内存状态偶然生效。
		watcher := permissionredis.NewWatcher(permissionredis.WatcherParams{
			RevisionSource: revisionSource,
			Engine:         engine,
		})
		watcher.CheckVersion(ctx)
		status := engine.ProjectionStatus()
		require.True(t, status.Ready())
		require.Equal(t, finalRevision, status.AppliedRevision)
		require.Equal(t, finalRevision, status.TargetRevision)
		allowed, enforceErr := engine.Enforce(ctx, userID, "/api/v1/recovery", "GET")
		require.NoError(t, enforceErr)
		require.True(t, allowed)
	}
}

type staticRBACUserRoleResolver struct {
	roleID uuid.UUID
}

func (r staticRBACUserRoleResolver) RolesForUser(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{r.roleID}, nil
}

func (staticRBACUserRoleResolver) InvalidateUserRole(uuid.UUID) {}
func (staticRBACUserRoleResolver) InvalidateAllUserRoles()      {}

func newRBACSyncPostgresClient(ctx context.Context, t *testing.T) *ent.Client {
	t.Helper()
	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	db, err := sql.Open("pgx", postgres.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm")
	require.NoError(t, err)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	require.NoError(t, client.Schema.Create(ctx))
	t.Cleanup(func() { _ = client.Close() })
	return client
}
