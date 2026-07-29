package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent/enttest"
)

func TestPermissionStoreUpsertAndQueryProjection(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000021")
	input := permissionapplication.SeedPermissionInput{PermissionID: permissionID, Name: "List Users", Description: "List", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"}

	created, inserted, err := store.UpsertPermission(ctx, input)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, permissionID, created.PermissionID)

	input.Name = "List Users Updated"
	updated, inserted, err := store.UpsertPermission(ctx, input)
	require.NoError(t, err)
	require.False(t, inserted)
	require.Equal(t, "List Users Updated", updated.Name)

	items, err := store.List(ctx, permissionapplication.ListPermissionsInput{Module: "user"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, permissionID, items[0].PermissionID)
}

func TestPermissionStoreListReturnsAllMatchingPermissionsInStableOrder(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	firstID := uuid.MustParse("018f0000-0000-7000-8000-000000000041")
	secondID := uuid.MustParse("018f0000-0000-7000-8000-000000000042")

	_, inserted, err := store.UpsertPermission(ctx, permissionapplication.SeedPermissionInput{PermissionID: secondID, Name: "Create User", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users"})
	require.NoError(t, err)
	require.True(t, inserted)
	_, inserted, err = store.UpsertPermission(ctx, permissionapplication.SeedPermissionInput{PermissionID: firstID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
	require.NoError(t, err)
	require.True(t, inserted)
	_, inserted, err = store.UpsertPermission(ctx, permissionapplication.SeedPermissionInput{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000043"), Name: "List Roles", Module: "role", HTTPMethod: "GET", PathTemplate: "/api/v1/roles"})
	require.NoError(t, err)
	require.True(t, inserted)

	items, err := store.List(ctx, permissionapplication.ListPermissionsInput{Module: "user"})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, firstID, items[0].PermissionID)
	require.Equal(t, secondID, items[1].PermissionID)
}

func TestPermissionStoreUpsertRejectsRouteOwnedByDifferentPermissionID(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	existingID := uuid.MustParse("018f0000-0000-7000-8000-000000000031")
	requestedID := uuid.MustParse("018f0000-0000-7000-8000-000000000032")
	route := permissionapplication.SeedPermissionInput{PermissionID: existingID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"}

	_, inserted, err := store.UpsertPermission(ctx, route)
	require.NoError(t, err)
	require.True(t, inserted)

	route.PermissionID = requestedID
	_, inserted, err = store.UpsertPermission(ctx, route)
	require.Error(t, err)
	require.False(t, inserted)
	require.ErrorContains(t, err, existingID.String())

	existing, err := store.GetByPermissionID(ctx, existingID)
	require.NoError(t, err)
	require.Equal(t, existingID, existing.PermissionID)
	_, err = store.GetByPermissionID(ctx, requestedID)
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
}

func TestPermissionStoreNotFound(t *testing.T) {
	store := newTestPermissionStore(t)
	_, err := store.GetByPermissionID(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000099"))
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
}

func newTestPermissionStore(t *testing.T) *PermissionStore {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:permission_store_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return NewPermissionStore(client)
}
