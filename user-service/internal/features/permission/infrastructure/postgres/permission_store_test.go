package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/user-service/ent/enttest"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionStoreDomainErrors(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()

	_, err := store.GetByPermissionID(ctx, uuid.MustParse("018f0000-0000-7000-8000-000000000099"))
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)

	input := permissionapplication.CreatePermissionInput{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}
	_, err = store.Create(ctx, input)
	require.NoError(t, err)
	input.PermissionID = uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	_, err = store.Create(ctx, input)
	require.ErrorIs(t, err, permissiondomain.ErrPermissionAlreadyExists)
}

func TestPermissionStoreListAndSetActive(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
	created, err := store.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)
	require.True(t, created.Active)
	require.False(t, created.IsSystem)

	err = store.SetActive(ctx, permissionID, false)
	require.NoError(t, err)
	disabled, err := store.GetByPermissionID(ctx, permissionID)
	require.NoError(t, err)
	require.False(t, disabled.Active)

	system := false
	items, hasNext, err := store.List(ctx, permissionapplication.ListPermissionsInput{Limit: 10, IsSystem: &system})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Len(t, items, 1)
	require.Equal(t, permissionID, items[0].PermissionID)
}

func TestPermissionStoreUpsertSystemPermission(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000021")
	input := permissionapplication.SeedPermissionInput{PermissionID: permissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}

	created, inserted, err := store.UpsertSystemPermission(ctx, input)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, permissionID, created.PermissionID)
	require.True(t, created.Active)
	require.True(t, created.IsSystem)
	err = store.SetActive(ctx, permissionID, false)
	require.NoError(t, err)
	input.Name = "List Users Updated"
	updated, inserted, err := store.UpsertSystemPermission(ctx, input)
	require.NoError(t, err)
	require.False(t, inserted)
	require.False(t, updated.Active)
	require.Equal(t, "List Users Updated", updated.Name)
	input.ReactivateSystem = true
	updated, inserted, err = store.UpsertSystemPermission(ctx, input)
	require.NoError(t, err)
	require.False(t, inserted)
	require.True(t, updated.Active)
}

func TestPermissionStoreUpsertSystemPermissionMatchesRouteIdentity(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	existingID := uuid.MustParse("018f0000-0000-7000-8000-000000000031")
	_, err := store.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: existingID, Name: "Old", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)

	created, inserted, err := store.UpsertSystemPermission(ctx, permissionapplication.SeedPermissionInput{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000032"), Name: "Create User", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)
	require.False(t, inserted)
	require.Equal(t, existingID, created.PermissionID)
	require.True(t, created.IsSystem)
	require.Equal(t, "Create User", created.Name)
}

func TestPermissionStoreUpdateAndSetActiveErrors(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000041")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000042")
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000049")

	_, err := store.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)
	_, err = store.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: otherPermissionID, Name: "Create Users", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)

	err = store.Update(ctx, permissionapplication.UpdatePermissionInput{PermissionID: permissionID, Name: "List Users Updated", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: false})
	require.NoError(t, err)
	updated, err := store.GetByPermissionID(ctx, permissionID)
	require.NoError(t, err)
	require.Equal(t, "List Users Updated", updated.Name)
	require.False(t, updated.Active)

	err = store.Update(ctx, permissionapplication.UpdatePermissionInput{PermissionID: missingPermissionID, Name: "Missing", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/missing", Active: true})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
	err = store.Update(ctx, permissionapplication.UpdatePermissionInput{PermissionID: permissionID, Name: "Duplicate", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionAlreadyExists)
	err = store.SetActive(ctx, missingPermissionID, true)
	require.ErrorIs(t, err, permissiondomain.ErrPermissionNotFound)
}

func newTestPermissionStore(t *testing.T) *PermissionStore {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:permission_store_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return NewPermissionStore(client)
}
