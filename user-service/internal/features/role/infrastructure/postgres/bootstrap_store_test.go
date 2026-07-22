package postgres

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/testing/containers"
	"github.com/aegiscore/user-service/ent"
	entuser "github.com/aegiscore/user-service/ent/user"
	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestBootstrapStoreSuccessAndDuplicateGuards(t *testing.T) {
	ctx, db, client := newBootstrapPostgresTestDB(t)
	store := NewBootstrapStore(db)
	roleID := uuid.MustParse(rbacbaseline.SuperAdminRoleID)
	createBootstrapRole(ctx, t, client, roleID, true, true)
	input := validBootstrapInput("initial-admin")

	result, err := store.BootstrapSuperAdmin(ctx, input)

	require.NoError(t, err)
	require.Equal(t, input.UserID, result.UserID)
	user := queryUserByUserID(ctx, t, client, input.UserID)
	require.Equal(t, int64(identity.UserStatusMustChangePassword), user.Status)
	require.Equal(t, input.PasswordHash, user.PasswordHash)
	require.Equal(t, input.Username, user.Username)
	roles, err := NewUserRoleStore(client).ListByUserID(ctx, input.UserID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleID}, roleIDs(roles))

	_, err = store.BootstrapSuperAdmin(ctx, validBootstrapInput("another-admin"))
	require.ErrorIs(t, err, rolebootstrap.ErrSuperAdminAlreadyBootstrapped)
}

func TestBootstrapStoreRejectsSoftDeletedBootstrapUser(t *testing.T) {
	ctx, db, client := newBootstrapPostgresTestDB(t)
	store := NewBootstrapStore(db)
	createBootstrapRole(ctx, t, client, uuid.MustParse(rbacbaseline.SuperAdminRoleID), true, true)
	createUserForTest(ctx, t, client, uuid.MustParse(rolebootstrap.BootstrapSuperAdminUserID), "deleted-bootstrap", true)

	_, err := store.BootstrapSuperAdmin(ctx, validBootstrapInput("initial-admin"))

	require.ErrorIs(t, err, rolebootstrap.ErrSuperAdminAlreadyBootstrapped)
}

func TestBootstrapStoreRejectsUsernameConflicts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		deleted bool
	}{
		{name: "normal user", deleted: false},
		{name: "soft deleted user", deleted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, db, client := newBootstrapPostgresTestDB(t)
			store := NewBootstrapStore(db)
			createBootstrapRole(ctx, t, client, uuid.MustParse(rbacbaseline.SuperAdminRoleID), true, true)
			createUserForTest(ctx, t, client, uuid.MustParse("018f0000-0000-7000-8000-000000009001"), "initial-admin", tc.deleted)

			_, err := store.BootstrapSuperAdmin(ctx, validBootstrapInput("initial-admin"))

			require.ErrorIs(t, err, rolebootstrap.ErrBootstrapUsernameAlreadyExists)
		})
	}
}

func TestBootstrapStoreRolePreconditions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		createRole bool
		isSystem   bool
		active     bool
		wantErr    error
	}{
		{name: "missing role", createRole: false, wantErr: roledomain.ErrRoleNotFound},
		{name: "not system", createRole: true, isSystem: false, active: true, wantErr: roledomain.ErrSystemRoleProtected},
		{name: "inactive", createRole: true, isSystem: true, active: false, wantErr: roledomain.ErrRoleInactive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, db, client := newBootstrapPostgresTestDB(t)
			if tc.createRole {
				createBootstrapRole(ctx, t, client, uuid.MustParse(rbacbaseline.SuperAdminRoleID), tc.isSystem, tc.active)
			}

			_, err := NewBootstrapStore(db).BootstrapSuperAdmin(ctx, validBootstrapInput("initial-admin"))

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestBootstrapStoreConcurrentExecution(t *testing.T) {
	ctx, db, client := newBootstrapPostgresTestDB(t)
	createBootstrapRole(ctx, t, client, uuid.MustParse(rbacbaseline.SuperAdminRoleID), true, true)
	store := NewBootstrapStore(db)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = store.BootstrapSuperAdmin(ctx, validBootstrapInput("initial-admin"))
		}(i)
	}
	wg.Wait()

	successes := 0
	duplicates := 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, rolebootstrap.ErrSuperAdminAlreadyBootstrapped) || errors.Is(err, rolebootstrap.ErrBootstrapUsernameAlreadyExists) {
			duplicates++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, len(errs)-1, duplicates)
}

func newBootstrapPostgresTestDB(t *testing.T) (context.Context, *sql.DB, *ent.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	db, err := sql.Open("pgx", postgres.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm")
	require.NoError(t, err)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	require.NoError(t, client.Schema.Create(ctx))
	t.Cleanup(func() { _ = client.Close() })
	return ctx, db, client
}

func validBootstrapInput(username string) rolebootstrap.BootstrapSuperAdminInput {
	return rolebootstrap.BootstrapSuperAdminInput{
		UserID:       uuid.MustParse(rolebootstrap.BootstrapSuperAdminUserID),
		RoleID:       uuid.MustParse(rbacbaseline.SuperAdminRoleID),
		Username:     username,
		Nickname:     username,
		PasswordHash: "bcrypt-hash",
		Status:       identity.UserStatusMustChangePassword,
	}
}

func createBootstrapRole(ctx context.Context, t *testing.T, client *ent.Client, roleID uuid.UUID, isSystem bool, active bool) {
	t.Helper()
	create := client.Role.Create().SetRoleID(roleID).SetName("Super Admin").SetDescription("all").SetActive(active).SetIsSystem(isSystem)
	_, err := create.Save(ctx)
	require.NoError(t, err)
}

func queryUserByUserID(ctx context.Context, t *testing.T, client *ent.Client, userID uuid.UUID) *ent.User {
	t.Helper()
	user, err := client.User.Query().Where(entuser.UserIDEQ(userID)).Only(ctx)
	require.NoError(t, err)
	return user
}
