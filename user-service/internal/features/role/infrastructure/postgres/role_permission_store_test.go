package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/testing/containers"
	"github.com/aegiscore/user-service/ent"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

func TestRolePermissionStorePostgresAddRechecksPermissionActiveState(t *testing.T) {
	client := newRolePermissionPostgresTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000402")
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})

	_, err := roleStore.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Operator", Active: true})
	require.NoError(t, err)
	createdPermission, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: "Create User", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)
	_, err = permissionStore.SetActive(ctx, permissionID, false)
	require.NoError(t, err)

	err = bindingStore.Add(ctx, roleID, roleapplication.PermissionReference{ID: createdPermission.ID, PermissionID: createdPermission.PermissionID})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	items, err := bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestRolePermissionStorePostgresReplaceRechecksPermissionActiveState(t *testing.T) {
	client := newRolePermissionPostgresTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	activePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000502")
	inactivePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000503")
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})

	_, err := roleStore.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Auditor", Active: true})
	require.NoError(t, err)
	activePermission, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: activePermissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)
	inactivePermission, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: inactivePermissionID, Name: "Delete User", Module: "user", HTTPMethod: "DELETE", PathTemplate: "/api/v1/users/:id", Active: true})
	require.NoError(t, err)
	_, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{{PermissionID: activePermissionID}})
	require.NoError(t, err)
	_, err = permissionStore.SetActive(ctx, inactivePermissionID, false)
	require.NoError(t, err)

	_, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{
		{ID: activePermission.ID, PermissionID: activePermission.PermissionID},
		{ID: inactivePermission.ID, PermissionID: inactivePermission.PermissionID},
	})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	items, err := bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, activePermissionID, items[0].PermissionID)
}

func newRolePermissionPostgresTestClient(t *testing.T) *ent.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	db, err := sql.Open("pgx", postgres.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	require.NoError(t, client.Schema.Create(ctx))
	t.Cleanup(func() { _ = client.Close() })
	return client
}
