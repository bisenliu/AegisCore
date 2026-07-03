package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/user-service/ent"
	"github.com/aegiscore/user-service/ent/enttest"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleStoreUpsertSystemRole(t *testing.T) {
	client := newRoleSeedTestClient(t)
	store := NewRoleStore(RoleStoreParams{Client: client})
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")
	input := roleapplication.SeedRoleInput{RoleID: roleID, Name: "Super Admin", Description: "all", Active: true, IsSystem: true}

	created, inserted, err := store.UpsertSystemRole(ctx, input)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, roleID, created.RoleID)
	require.True(t, created.Active)
	require.True(t, created.IsSystem)
	_, err = store.SetActive(ctx, roleID, false)
	require.NoError(t, err)

	input.Name = "Super Admin Updated"
	updated, inserted, err := store.UpsertSystemRole(ctx, input)
	require.NoError(t, err)
	require.False(t, inserted)
	require.False(t, updated.Active)
	require.Equal(t, "Super Admin Updated", updated.Name)

	input.ReactivateSystem = true
	updated, inserted, err = store.UpsertSystemRole(ctx, input)
	require.NoError(t, err)
	require.False(t, inserted)
	require.True(t, updated.Active)
}

func TestRolePermissionStoreSeedEnsureAndSync(t *testing.T) {
	client := newRoleSeedTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000202")
	extraPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")

	_, _, err := roleStore.UpsertSystemRole(ctx, roleapplication.SeedRoleInput{RoleID: roleID, Name: "System", Active: true, IsSystem: true})
	require.NoError(t, err)
	_, err = permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: "List", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true, IsSystem: true})
	require.NoError(t, err)
	_, err = permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: extraPermissionID, Name: "Create", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true, IsSystem: true})
	require.NoError(t, err)

	added, err := bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{permissionID, extraPermissionID})
	require.NoError(t, err)
	require.Equal(t, 2, added)

	added, err = bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{permissionID, extraPermissionID, permissionID})
	require.NoError(t, err)
	require.Zero(t, added)

	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	_, err = bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{missingPermissionID})
	require.ErrorIs(t, err, roledomain.ErrRolePermissionNotFound)

	added, removed, err := bindingStore.SyncSystemBindings(ctx, roleID, []uuid.UUID{permissionID})
	require.NoError(t, err)
	require.Zero(t, added)
	require.Equal(t, 1, removed)
}

func TestUserRoleStoreAssignRoleIdempotent(t *testing.T) {
	client := newRoleSeedTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	userRoleStore := NewUserRoleStore(UserRoleStoreParams{Client: client})
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")
	_, err := client.User.Create().SetUserID(userID).SetNickname("Admin").SetUsername("admin@example.com").SetPasswordHash("hash").Save(ctx)
	require.NoError(t, err)
	_, _, err = roleStore.UpsertSystemRole(ctx, roleapplication.SeedRoleInput{RoleID: roleID, Name: "System", Active: true, IsSystem: true})
	require.NoError(t, err)

	added, err := userRoleStore.AssignRole(ctx, userID, roleID)
	require.NoError(t, err)
	require.True(t, added)

	added, err = userRoleStore.AssignRole(ctx, userID, roleID)
	require.NoError(t, err)
	require.False(t, added)
}

func newRoleSeedTestClient(t *testing.T) *ent.Client {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:role_seed_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return client
}
