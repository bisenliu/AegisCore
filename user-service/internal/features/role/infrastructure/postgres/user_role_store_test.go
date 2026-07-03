package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/user-service/ent"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestUserRoleStoreAddListAndRemove(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	userRoleStore := NewUserRoleStore(UserRoleStoreParams{Client: client})
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000002001")
	softDeletedUserID := uuid.MustParse("018f0000-0000-7000-8000-000000002002")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000002101")
	missingUserID := uuid.MustParse("018f0000-0000-7000-8000-000000002099")
	missingRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000002199")
	createUserForTest(ctx, t, client, userID, "user-role-add@example.com", false)
	createUserForTest(ctx, t, client, softDeletedUserID, "deleted-user-role-add@example.com", true)
	createRoleForTest(ctx, t, roleStore, roleID, "Operator", true, false)

	err := userRoleStore.Add(ctx, userID, roleID)
	require.NoError(t, err)
	err = userRoleStore.Add(ctx, userID, roleID)
	require.ErrorIs(t, err, roledomain.ErrUserRoleAlreadyExists)

	items, err := userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleID}, roleIDs(items))
	_, err = userRoleStore.ListByUserID(ctx, missingUserID)
	require.ErrorIs(t, err, identity.ErrUserNotFound)
	_, err = userRoleStore.ListByUserID(ctx, softDeletedUserID)
	require.ErrorIs(t, err, identity.ErrUserNotFound)

	err = userRoleStore.Add(ctx, missingUserID, roleID)
	require.ErrorIs(t, err, identity.ErrUserNotFound)
	err = userRoleStore.Add(ctx, userID, missingRoleID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	err = userRoleStore.Remove(ctx, userID, roleID)
	require.NoError(t, err)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, items)
	err = userRoleStore.Remove(ctx, userID, roleID)
	require.ErrorIs(t, err, roledomain.ErrUserRoleNotFound)
	err = userRoleStore.Remove(ctx, missingUserID, roleID)
	require.ErrorIs(t, err, identity.ErrUserNotFound)
	err = userRoleStore.Remove(ctx, userID, missingRoleID)
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
}

func TestUserRoleStoreReplace(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	userRoleStore := NewUserRoleStore(UserRoleStoreParams{Client: client})
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000002201")
	roleAID := uuid.MustParse("018f0000-0000-7000-8000-000000002301")
	roleBID := uuid.MustParse("018f0000-0000-7000-8000-000000002302")
	roleCID := uuid.MustParse("018f0000-0000-7000-8000-000000002303")
	missingUserID := uuid.MustParse("018f0000-0000-7000-8000-000000002299")
	missingRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000002399")
	createUserForTest(ctx, t, client, userID, "user-role-replace@example.com", false)
	createRoleForTest(ctx, t, roleStore, roleAID, "Alpha", true, false)
	createRoleForTest(ctx, t, roleStore, roleBID, "Beta", true, false)
	createRoleForTest(ctx, t, roleStore, roleCID, "Gamma", true, false)
	require.NoError(t, userRoleStore.Add(ctx, userID, roleAID))

	replaced, err := userRoleStore.Replace(ctx, userID, []uuid.UUID{roleBID, roleCID})
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{roleBID, roleCID}, roleIDs(replaced))
	items, err := userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleBID, roleCID}, roleIDs(items))

	_, err = userRoleStore.Replace(ctx, missingUserID, []uuid.UUID{roleAID})
	require.ErrorIs(t, err, identity.ErrUserNotFound)

	_, err = userRoleStore.Replace(ctx, userID, []uuid.UUID{roleBID, missingRoleID})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleBID, roleCID}, roleIDs(items))

	_, err = userRoleStore.Replace(ctx, userID, []uuid.UUID{roleBID, roleBID})
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleBID, roleCID}, roleIDs(items))

	replaced, err = userRoleStore.Replace(ctx, userID, nil)
	require.NoError(t, err)
	require.Empty(t, replaced)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, items)
}

func createUserForTest(ctx context.Context, t *testing.T, client *ent.Client, userID uuid.UUID, username string, deleted bool) *ent.User {
	t.Helper()
	create := client.User.Create().
		SetUserID(userID).
		SetNickname(username).
		SetUsername(username).
		SetPasswordHash("hash")
	if deleted {
		create.SetDeletedAt(1)
	}
	user, err := create.Save(ctx)
	require.NoError(t, err)
	return user
}
