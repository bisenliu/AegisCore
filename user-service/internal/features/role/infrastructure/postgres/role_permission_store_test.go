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
	entrole "github.com/aegiscore/user-service/ent/role"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRolePermissionStoreDefaultListRemoveAndSyncSystemBindings(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000010001")
	keepPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000010002")
	removePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000010003")
	addPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000010004")
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000010005")
	missingRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000010006")

	createRoleForTest(ctx, t, roleStore, roleID, "Default System Role", true, true)
	role, err := client.Role.Query().Where(entrole.RoleIDEQ(roleID)).Only(ctx)
	require.NoError(t, err)
	keepPermission := createPermissionForTest(ctx, t, permissionStore, keepPermissionID, "Keep Default System", "GET", "/api/v1/default-system/keep", true)
	removePermission := createPermissionForTest(ctx, t, permissionStore, removePermissionID, "Remove Default System", "GET", "/api/v1/default-system/remove", true)
	createPermissionForTest(ctx, t, permissionStore, addPermissionID, "Add Default System", "GET", "/api/v1/default-system/add", true)
	require.NoError(t, client.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(keepPermission.ID).Exec(ctx))
	require.NoError(t, client.RolePermission.Create().SetRoleID(role.ID).SetPermissionID(removePermission.ID).Exec(ctx))

	items, err := bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{keepPermissionID, removePermissionID}, permissionIDsForTest(items))
	_, err = bindingStore.ListByRoleID(ctx, missingRoleID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	added, removed, err := bindingStore.SyncSystemBindings(ctx, roleID, []uuid.UUID{keepPermissionID, addPermissionID})
	require.NoError(t, err)
	require.Equal(t, 1, added)
	require.Equal(t, 1, removed)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{keepPermissionID, addPermissionID}, permissionIDsForTest(items))

	_, _, err = bindingStore.SyncSystemBindings(ctx, roleID, []uuid.UUID{missingPermissionID})
	require.ErrorIs(t, err, roledomain.ErrRolePermissionNotFound)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{keepPermissionID, addPermissionID}, permissionIDsForTest(items))
	_, _, err = bindingStore.SyncSystemBindings(ctx, missingRoleID, []uuid.UUID{keepPermissionID})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	added, removed, err = bindingStore.SyncSystemBindings(ctx, roleID, nil)
	require.NoError(t, err)
	require.Zero(t, added)
	require.Equal(t, 2, removed)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Empty(t, items)

	added, err = bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{keepPermissionID, keepPermissionID, addPermissionID})
	require.NoError(t, err)
	require.Equal(t, 2, added)
	added, err = bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{keepPermissionID, addPermissionID})
	require.NoError(t, err)
	require.Zero(t, added)

	require.NoError(t, bindingStore.Remove(ctx, roleID, addPermissionID))
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{keepPermissionID}, permissionIDsForTest(items))
	err = bindingStore.Remove(ctx, roleID, addPermissionID)
	require.ErrorIs(t, err, roledomain.ErrRolePermissionNotFound)
	err = bindingStore.Remove(ctx, missingRoleID, keepPermissionID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	err = bindingStore.Remove(ctx, roleID, missingPermissionID)
	require.ErrorIs(t, err, roledomain.ErrRolePermissionNotFound)
}

func TestPermissionLookupDefaultGetActiveByPermissionID(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	lookup := NewPermissionLookup(PermissionLookupParams{Store: permissionStore})
	activePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000011001")
	inactivePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000011002")
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000011003")
	activePermission := createPermissionForTest(ctx, t, permissionStore, activePermissionID, "Lookup Active", "GET", "/api/v1/lookup/active", true)
	createPermissionForTest(ctx, t, permissionStore, inactivePermissionID, "Lookup Inactive", "GET", "/api/v1/lookup/inactive", false)

	permission, err := lookup.GetActiveByPermissionID(ctx, activePermissionID)
	require.NoError(t, err)
	require.Equal(t, activePermission.PermissionID, permission.PermissionID)
	require.Equal(t, activePermission.PathTemplate, permission.PathTemplate)

	_, err = lookup.GetActiveByPermissionID(ctx, inactivePermissionID)
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	_, err = lookup.GetActiveByPermissionID(ctx, missingPermissionID)
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
}

func TestRolePermissionStoreMappingHelpers(t *testing.T) {
	first := uuid.MustParse("018f0000-0000-7000-8000-000000012001")
	second := uuid.MustParse("018f0000-0000-7000-8000-000000012002")

	require.Nil(t, toPermissionReference(nil))
	require.Equal(t, []uuid.UUID{first, second}, permissionReferenceIDs([]roleapplication.PermissionReference{
		{PermissionID: first},
		{PermissionID: second},
		{PermissionID: first},
	}))
}

func TestRolePermissionStorePostgresAddListAndRemove(t *testing.T) {
	client := newRolePermissionPostgresTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000601")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000602")
	missingRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000699")
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000698")
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})
	createRoleForTest(ctx, t, roleStore, roleID, "Permission Operator", true, false)
	permission := createPermissionForTest(ctx, t, permissionStore, permissionID, "List Operators", "GET", "/api/v1/operators", true)

	err := bindingStore.Add(ctx, roleID, permission)
	require.NoError(t, err)
	err = bindingStore.Add(ctx, roleID, permission)
	require.ErrorIs(t, err, roledomain.ErrRolePermissionAlreadyExists)

	items, err := bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionID}, permissionIDsForTest(items))
	_, err = bindingStore.ListByRoleID(ctx, missingRoleID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	err = bindingStore.Add(ctx, missingRoleID, permission)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	err = bindingStore.Add(ctx, roleID, roleapplication.PermissionReference{PermissionID: missingPermissionID})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)

	err = bindingStore.Remove(ctx, roleID, permissionID)
	require.NoError(t, err)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Empty(t, items)
	err = bindingStore.Remove(ctx, roleID, permissionID)
	require.ErrorIs(t, err, roledomain.ErrRolePermissionNotFound)
	err = bindingStore.Remove(ctx, missingRoleID, permissionID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	err = bindingStore.Remove(ctx, roleID, missingPermissionID)
	require.ErrorIs(t, err, roledomain.ErrRolePermissionNotFound)
}

func TestRolePermissionStorePostgresReplace(t *testing.T) {
	client := newRolePermissionPostgresTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	missingRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000799")
	permissionAID := uuid.MustParse("018f0000-0000-7000-8000-000000000702")
	permissionBID := uuid.MustParse("018f0000-0000-7000-8000-000000000703")
	permissionCID := uuid.MustParse("018f0000-0000-7000-8000-000000000704")
	inactivePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000705")
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000798")
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})
	createRoleForTest(ctx, t, roleStore, roleID, "Permission Auditor", true, false)
	permissionA := createPermissionForTest(ctx, t, permissionStore, permissionAID, "Read Reports", "GET", "/api/v1/reports", true)
	permissionB := createPermissionForTest(ctx, t, permissionStore, permissionBID, "Create Reports", "POST", "/api/v1/reports", true)
	permissionC := createPermissionForTest(ctx, t, permissionStore, permissionCID, "Delete Reports", "DELETE", "/api/v1/reports/:id", true)
	inactivePermission := createPermissionForTest(ctx, t, permissionStore, inactivePermissionID, "Archive Reports", "PATCH", "/api/v1/reports/:id/archive", false)

	replaced, err := bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{permissionA, permissionB})
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{permissionAID, permissionBID}, permissionIDsForTest(replaced))
	items, err := bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{permissionAID, permissionBID}, permissionIDsForTest(items))

	replaced, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{permissionC, permissionC})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionCID}, permissionIDsForTest(replaced))
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionCID}, permissionIDsForTest(items))

	_, err = bindingStore.Replace(ctx, missingRoleID, []roleapplication.PermissionReference{permissionA})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	_, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{permissionA, inactivePermission})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionCID}, permissionIDsForTest(items))

	_, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{{PermissionID: missingPermissionID}})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionCID}, permissionIDsForTest(items))

	replaced, err = bindingStore.Replace(ctx, roleID, nil)
	require.NoError(t, err)
	require.Empty(t, replaced)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Empty(t, items)
}

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
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm")
	require.NoError(t, err)

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	require.NoError(t, client.Schema.Create(ctx))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func createPermissionForTest(ctx context.Context, t *testing.T, store *permissionpostgres.PermissionStore, permissionID uuid.UUID, name string, method string, path string, active bool) roleapplication.PermissionReference {
	t.Helper()
	permission, err := store.Create(ctx, permissionapplication.CreatePermissionInput{
		PermissionID: permissionID,
		Name:         name,
		Module:       "role",
		HTTPMethod:   method,
		PathTemplate: path,
		Active:       active,
	})
	require.NoError(t, err)
	return roleapplication.PermissionReference{
		ID:           permission.ID,
		PermissionID: permission.PermissionID,
		Name:         permission.Name,
		Description:  permission.Description,
		Module:       permission.Module,
		HTTPMethod:   permission.HTTPMethod,
		PathTemplate: permission.PathTemplate,
		Active:       permission.Active,
		IsSystem:     permission.IsSystem,
		CreatedAt:    permission.CreatedAt,
		UpdatedAt:    permission.UpdatedAt,
	}
}

func permissionIDsForTest(permissions []roleapplication.PermissionReference) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, permission.PermissionID)
	}
	return result
}
