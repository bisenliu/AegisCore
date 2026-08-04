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
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
)

func TestRolePermissionStoreDefaultListRemoveAndSyncSystemBindings(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(client)
	permissionStore := permissionpostgres.NewPermissionStore(client)
	bindingStore := NewRolePermissionStore(client)
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

}

func TestPermissionLookup(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	permissionStore := permissionpostgres.NewPermissionStore(client)
	lookup := NewPermissionLookup(permissionStore)
	firstPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000011001")
	secondPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000011002")
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000011003")
	firstPermission := createPermissionForTest(ctx, t, permissionStore, firstPermissionID, "Lookup First", "GET", "/api/v1/lookup/first", true)
	secondPermission := createPermissionForTest(ctx, t, permissionStore, secondPermissionID, "Lookup Second", "GET", "/api/v1/lookup/second", true)

	permission, err := lookup.GetByPermissionID(ctx, firstPermissionID)
	require.NoError(t, err)
	require.Equal(t, firstPermission.PermissionID, permission.PermissionID)
	require.Equal(t, firstPermission.PathTemplate, permission.PathTemplate)

	_, err = lookup.GetByPermissionID(ctx, missingPermissionID)
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)

	empty, err := lookup.GetByPermissionIDs(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Empty(t, empty)

	single, err := lookup.GetByPermissionIDs(ctx, []uuid.UUID{firstPermissionID})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{firstPermissionID}, permissionIDsForTest(single))

	permissions, err := lookup.GetByPermissionIDs(ctx, []uuid.UUID{secondPermissionID, firstPermissionID, secondPermissionID})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{secondPermissionID, firstPermissionID}, permissionIDsForTest(permissions))
	require.Equal(t, secondPermission.PathTemplate, permissions[0].PathTemplate)
	require.Equal(t, firstPermission.PathTemplate, permissions[1].PathTemplate)

	permissions, err = lookup.GetByPermissionIDs(ctx, []uuid.UUID{firstPermissionID, missingPermissionID})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	require.Nil(t, permissions)
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
	roleStore := NewRoleStore(client)
	permissionStore := permissionpostgres.NewPermissionStore(client)
	bindingStore := NewRolePermissionStore(client)
	createRoleForTest(ctx, t, roleStore, roleID, "Permission Operator", true, false)
	permission := createPermissionForTest(ctx, t, permissionStore, permissionID, "List Operators", "GET", "/api/v1/operators", true)

	_, err := bindingStore.Add(ctx, roleID, permission, permissionPolicyChange("role_permission_added", roleID, permissionID))
	require.NoError(t, err)
	_, err = bindingStore.Add(ctx, roleID, permission, permissionPolicyChange("role_permission_added", roleID, permissionID))
	require.ErrorIs(t, err, roledomain.ErrRolePermissionAlreadyExists)

	items, err := bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionID}, permissionIDsForTest(items))
	_, err = bindingStore.ListByRoleID(ctx, missingRoleID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	_, err = bindingStore.Add(ctx, missingRoleID, permission, permissionPolicyChange("role_permission_added", missingRoleID, permissionID))
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	_, err = bindingStore.Add(ctx, roleID, roleapplication.PermissionReference{PermissionID: missingPermissionID}, permissionPolicyChange("role_permission_added", roleID, missingPermissionID))
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)

	_, err = bindingStore.Remove(ctx, roleID, permissionID, permissionPolicyChange("role_permission_removed", roleID, permissionID))
	require.NoError(t, err)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Empty(t, items)
	_, err = bindingStore.Remove(ctx, roleID, permissionID, permissionPolicyChange("role_permission_removed", roleID, permissionID))
	require.ErrorIs(t, err, roledomain.ErrRolePermissionNotFound)
	_, err = bindingStore.Remove(ctx, missingRoleID, permissionID, permissionPolicyChange("role_permission_removed", missingRoleID, permissionID))
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	_, err = bindingStore.Remove(ctx, roleID, missingPermissionID, permissionPolicyChange("role_permission_removed", roleID, missingPermissionID))
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
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000798")
	roleStore := NewRoleStore(client)
	permissionStore := permissionpostgres.NewPermissionStore(client)
	bindingStore := NewRolePermissionStore(client)
	createRoleForTest(ctx, t, roleStore, roleID, "Permission Auditor", true, false)
	permissionA := createPermissionForTest(ctx, t, permissionStore, permissionAID, "Read Reports", "GET", "/api/v1/reports", true)
	permissionB := createPermissionForTest(ctx, t, permissionStore, permissionBID, "Create Reports", "POST", "/api/v1/reports", true)
	permissionC := createPermissionForTest(ctx, t, permissionStore, permissionCID, "Delete Reports", "DELETE", "/api/v1/reports/:id", true)

	replaced, err := bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{permissionA, permissionB}, permissionPolicyChange("role_permissions_replaced", roleID, uuid.Nil))
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{permissionAID, permissionBID}, permissionIDsForTest(replaced.Items))
	items, err := bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{permissionAID, permissionBID}, permissionIDsForTest(items))

	replaced, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{permissionC, permissionC}, permissionPolicyChange("role_permissions_replaced", roleID, uuid.Nil))
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionCID}, permissionIDsForTest(replaced.Items))
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionCID}, permissionIDsForTest(items))

	_, err = bindingStore.Replace(ctx, missingRoleID, []roleapplication.PermissionReference{permissionA}, permissionPolicyChange("role_permissions_replaced", missingRoleID, uuid.Nil))
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	_, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{{PermissionID: missingPermissionID}}, permissionPolicyChange("role_permissions_replaced", roleID, uuid.Nil))
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{permissionCID}, permissionIDsForTest(items))

	replaced, err = bindingStore.Replace(ctx, roleID, nil, permissionPolicyChange("role_permissions_replaced", roleID, uuid.Nil))
	require.NoError(t, err)
	require.Empty(t, replaced.Items)
	items, err = bindingStore.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Empty(t, items)
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

func createPermissionForTest(ctx context.Context, t *testing.T, store *permissionpostgres.PermissionStore, permissionID uuid.UUID, name string, method string, path string, _ bool) roleapplication.PermissionReference {
	t.Helper()
	permission, _, err := store.UpsertPermission(ctx, permissionapplication.SeedPermissionInput{
		PermissionID: permissionID,
		Name:         name,
		Module:       "role",
		HTTPMethod:   method,
		PathTemplate: path,
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

func permissionPolicyChange(reason string, roleID uuid.UUID, permissionID uuid.UUID) roleapplication.PolicyChange {
	return roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: reason, RoleID: roleID, PermissionID: permissionID}
}
