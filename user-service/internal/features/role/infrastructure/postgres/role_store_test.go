package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleStoreCRUDAndDomainErrors(t *testing.T) {
	client := newRoleStoreTestClient(t)
	store := NewRoleStore(client)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000001001")
	missingRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000001099")

	roles, err := store.GetByRoleIDs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, roles)
	_, err = store.GetByRoleID(ctx, missingRoleID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	created, err := store.Create(ctx, roleapplication.CreateRoleInput{
		RoleID:      roleID,
		Name:        "Operator",
		Description: "operate user resources",
		Active:      true,
	})
	require.NoError(t, err)
	require.Equal(t, roleID, created.RoleID)
	require.Equal(t, "Operator", created.Name)
	require.True(t, created.Active)
	require.False(t, created.IsSystem)

	got, err := store.GetByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, roleID, got.RoleID)

	batch, err := store.GetByRoleIDs(ctx, []uuid.UUID{roleID})
	require.NoError(t, err)
	require.Len(t, batch, 1)
	require.Equal(t, roleID, batch[0].RoleID)
	_, err = store.GetByRoleIDs(ctx, []uuid.UUID{roleID, missingRoleID})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	_, err = store.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Duplicate", Active: true})
	require.ErrorIs(t, err, roledomain.ErrRoleAlreadyExists)

	err = store.Update(ctx, roleapplication.UpdateRoleInput{RoleID: missingRoleID, Name: "Missing", Active: true})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	err = store.Update(ctx, roleapplication.UpdateRoleInput{
		RoleID:      roleID,
		Name:        "Operator Updated",
		Description: "updated description",
		Active:      false,
	})
	require.NoError(t, err)
	updated, err := store.GetByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.Equal(t, "Operator Updated", updated.Name)
	require.Equal(t, "updated description", updated.Description)
	require.False(t, updated.Active)

	err = store.SetActive(ctx, roleID, true)
	require.NoError(t, err)
	activated, err := store.GetByRoleID(ctx, roleID)
	require.NoError(t, err)
	require.True(t, activated.Active)
	err = store.SetActive(ctx, missingRoleID, true)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

}

func TestRoleStoreListFiltersAndPagination(t *testing.T) {
	client := newRoleStoreTestClient(t)
	store := NewRoleStore(client)
	ctx := context.Background()
	roleAID := uuid.MustParse("018f0000-0000-7000-8000-000000001101")
	roleBID := uuid.MustParse("018f0000-0000-7000-8000-000000001102")
	roleCID := uuid.MustParse("018f0000-0000-7000-8000-000000001103")
	createRoleForTest(ctx, t, store, roleAID, "Alpha", true, false)
	createRoleForTest(ctx, t, store, roleBID, "Beta", false, false)
	createRoleForTest(ctx, t, store, roleCID, "Gamma", true, true)

	items, hasNext, err := store.List(ctx, roleapplication.ListRolesInput{Limit: 2})
	require.NoError(t, err)
	require.True(t, hasNext)
	require.Equal(t, []uuid.UUID{roleAID, roleBID}, roleIDs(items))

	items, hasNext, err = store.List(ctx, roleapplication.ListRolesInput{AfterRoleID: &roleAID, Limit: 10})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Equal(t, []uuid.UUID{roleBID, roleCID}, roleIDs(items))

	active := true
	items, hasNext, err = store.List(ctx, roleapplication.ListRolesInput{Active: &active, Limit: 10})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Equal(t, []uuid.UUID{roleAID, roleCID}, roleIDs(items))

	system := true
	items, hasNext, err = store.List(ctx, roleapplication.ListRolesInput{IsSystem: &system, Limit: 10})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Equal(t, []uuid.UUID{roleCID}, roleIDs(items))

	inactive := false
	items, hasNext, err = store.List(ctx, roleapplication.ListRolesInput{Active: &inactive, IsSystem: &system, Limit: 10})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Empty(t, items)
}

func createRoleForTest(ctx context.Context, t *testing.T, store *RoleStore, roleID uuid.UUID, name string, active bool, system bool) roledomain.Role {
	t.Helper()
	if system {
		role, _, err := store.UpsertSystemRole(ctx, roleapplication.SeedRoleInput{RoleID: roleID, Name: name, Description: name + " role", Active: active, ReactivateSystem: true})
		require.NoError(t, err)
		require.NotNil(t, role)
		return *role
	}
	role, err := store.Create(ctx, roleapplication.CreateRoleInput{
		RoleID:      roleID,
		Name:        name,
		Description: name + " role",
		Active:      active,
	})
	require.NoError(t, err)
	require.NotNil(t, role)
	return *role
}

func roleIDs(roles []roledomain.Role) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(roles))
	for _, role := range roles {
		result = append(result, role.RoleID)
	}
	return result
}
