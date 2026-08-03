package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestUserRoleStoreAddListAndRemove(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(client)
	userRoleStore := NewUserRoleStore(client)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000002001")
	softDeletedUserID := uuid.MustParse("018f0000-0000-7000-8000-000000002002")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000002101")
	missingUserID := uuid.MustParse("018f0000-0000-7000-8000-000000002099")
	missingRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000002199")
	createUserForTest(ctx, t, client, userID, "user-role-add@example.com", false)
	createUserForTest(ctx, t, client, softDeletedUserID, "deleted-user-role-add@example.com", true)
	createRoleForTest(ctx, t, roleStore, roleID, "Operator", true, false)

	_, err := userRoleStore.Add(ctx, userID, roleID, userRolePolicyChange("user_role_added", userID, roleID))
	require.NoError(t, err)
	_, err = userRoleStore.Add(ctx, userID, roleID, userRolePolicyChange("user_role_added", userID, roleID))
	require.ErrorIs(t, err, roledomain.ErrUserRoleAlreadyExists)

	items, err := userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleID}, roleIDs(items))
	_, err = userRoleStore.ListByUserID(ctx, missingUserID)
	require.ErrorIs(t, err, identity.ErrUserNotFound)
	_, err = userRoleStore.ListByUserID(ctx, softDeletedUserID)
	require.ErrorIs(t, err, identity.ErrUserNotFound)

	_, err = userRoleStore.Add(ctx, missingUserID, roleID, userRolePolicyChange("user_role_added", missingUserID, roleID))
	require.ErrorIs(t, err, identity.ErrUserNotFound)
	_, err = userRoleStore.Add(ctx, userID, missingRoleID, userRolePolicyChange("user_role_added", userID, missingRoleID))
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)

	_, err = userRoleStore.Remove(ctx, userID, roleID, userRolePolicyChange("user_role_removed", userID, roleID))
	require.NoError(t, err)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, items)
	_, err = userRoleStore.Remove(ctx, userID, roleID, userRolePolicyChange("user_role_removed", userID, roleID))
	require.ErrorIs(t, err, roledomain.ErrUserRoleNotFound)
	_, err = userRoleStore.Remove(ctx, missingUserID, roleID, userRolePolicyChange("user_role_removed", missingUserID, roleID))
	require.ErrorIs(t, err, identity.ErrUserNotFound)
	_, err = userRoleStore.Remove(ctx, userID, missingRoleID, userRolePolicyChange("user_role_removed", userID, missingRoleID))
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
}

func TestUserRoleStoreReplace(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(client)
	userRoleStore := NewUserRoleStore(client)
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
	_, err := userRoleStore.Add(ctx, userID, roleAID, userRolePolicyChange("user_role_added", userID, roleAID))
	require.NoError(t, err)

	replaced, err := userRoleStore.Replace(ctx, userID, []uuid.UUID{roleBID, roleCID}, userRolePolicyChange("user_roles_replaced", userID, uuid.Nil))
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{roleBID, roleCID}, roleIDs(replaced.Items))
	items, err := userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleBID, roleCID}, roleIDs(items))

	_, err = userRoleStore.Replace(ctx, missingUserID, []uuid.UUID{roleAID}, userRolePolicyChange("user_roles_replaced", missingUserID, uuid.Nil))
	require.ErrorIs(t, err, identity.ErrUserNotFound)

	_, err = userRoleStore.Replace(ctx, userID, []uuid.UUID{roleBID, missingRoleID}, userRolePolicyChange("user_roles_replaced", userID, uuid.Nil))
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleBID, roleCID}, roleIDs(items))

	_, err = userRoleStore.Replace(ctx, userID, []uuid.UUID{roleBID, roleBID}, userRolePolicyChange("user_roles_replaced", userID, uuid.Nil))
	require.ErrorIs(t, err, roledomain.ErrRoleNotFound)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{roleBID, roleCID}, roleIDs(items))

	replaced, err = userRoleStore.Replace(ctx, userID, nil, userRolePolicyChange("user_roles_replaced", userID, uuid.Nil))
	require.NoError(t, err)
	require.Empty(t, replaced.Items)
	items, err = userRoleStore.ListByUserID(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, items)
}

func userRolePolicyChange(reason string, userID uuid.UUID, roleID uuid.UUID) roleapplication.PolicyChange {
	return roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindUserRoleChanged, Reason: reason, UserID: userID, RoleID: roleID}
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
