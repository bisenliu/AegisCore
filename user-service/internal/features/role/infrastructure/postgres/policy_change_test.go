package postgres

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrbacpolicyoutboxevent "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyoutboxevent"
	entrbacpolicyrevision "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyrevision"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
)

func TestOnlineRoleMutationsAppendCommittedRevisionAndPendingOutbox(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	roles := NewRoleStore(client)
	rolePermissions := NewRolePermissionStore(client)
	userRoles := NewUserRoleStore(client)
	permissions := permissionpostgres.NewPermissionStore(client)
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000020001")
	otherRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000020002")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000020003")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000020004")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000020005")

	created, err := roles.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Policy Role", Active: true}, rolePolicyChange("role_created", roleID))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, created.Revision, roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: "role_created", RoleID: roleID})

	updated, err := roles.Update(ctx, roleapplication.UpdateRoleInput{RoleID: roleID, Name: "Policy Role Updated", Active: true}, rolePolicyChange("role_updated", roleID))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, updated.Revision, rolePolicyChange("role_updated", roleID))

	status, err := roles.SetActive(ctx, roleID, false, rolePolicyChange("role_active_changed", roleID))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, status.Revision, rolePolicyChange("role_active_changed", roleID))

	createRoleForTest(ctx, t, roles, otherRoleID, "Other Policy Role", true, false)
	permission := createPermissionForTest(ctx, t, permissions, permissionID, "Policy Permission", "GET", "/api/v1/policy", true)
	otherPermission := createPermissionForTest(ctx, t, permissions, otherPermissionID, "Other Policy Permission", "POST", "/api/v1/policy", true)
	createUserForTest(ctx, t, client, userID, "policy-user@example.com", false)

	permissionAdded, err := rolePermissions.Add(ctx, roleID, permission, permissionPolicyChange("role_permission_added", roleID, permissionID))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, permissionAdded.Revision, permissionPolicyChange("role_permission_added", roleID, permissionID))
	permissionReplaced, err := rolePermissions.Replace(ctx, roleID, []roleapplication.PermissionReference{otherPermission}, permissionPolicyChange("role_permissions_replaced", roleID, uuid.Nil))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, permissionReplaced.Revision, permissionPolicyChange("role_permissions_replaced", roleID, uuid.Nil))
	permissionRemoved, err := rolePermissions.Remove(ctx, roleID, otherPermissionID, permissionPolicyChange("role_permission_removed", roleID, otherPermissionID))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, permissionRemoved.Revision, permissionPolicyChange("role_permission_removed", roleID, otherPermissionID))

	userRoleAdded, err := userRoles.Add(ctx, userID, roleID, userRolePolicyChange("user_role_added", userID, roleID))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, userRoleAdded.Revision, userRolePolicyChange("user_role_added", userID, roleID))
	userRolesReplaced, err := userRoles.Replace(ctx, userID, []uuid.UUID{otherRoleID}, userRolePolicyChange("user_roles_replaced", userID, uuid.Nil))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, userRolesReplaced.Revision, userRolePolicyChange("user_roles_replaced", userID, uuid.Nil))
	userRoleRemoved, err := userRoles.Remove(ctx, userID, otherRoleID, userRolePolicyChange("user_role_removed", userID, otherRoleID))
	require.NoError(t, err)
	assertPolicyFact(ctx, t, client, userRoleRemoved.Revision, userRolePolicyChange("user_role_removed", userID, otherRoleID))

	committed := []int64{created.Revision, updated.Revision, status.Revision, permissionAdded.Revision, permissionReplaced.Revision, permissionRemoved.Revision, userRoleAdded.Revision, userRolesReplaced.Revision, userRoleRemoved.Revision}
	for index := 1; index < len(committed); index++ {
		require.Greater(t, committed[index], committed[index-1])
	}
}

func TestPolicyFactFailuresRollbackBusinessMutation(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	store := NewRoleStore(client)
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000021001")
	created, err := store.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Before Failure", Active: true}, rolePolicyChange("role_created", roleID))
	require.NoError(t, err)
	baseRevisionCount, err := client.RbacPolicyRevision.Query().Count(ctx)
	require.NoError(t, err)
	baseOutboxCount, err := client.RbacPolicyOutboxEvent.Query().Count(ctx)
	require.NoError(t, err)

	t.Run("business mutation failure", func(t *testing.T) {
		_, err := store.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Duplicate", Active: true}, rolePolicyChange("role_created", roleID))
		require.Error(t, err)
		assertRoleAndFactCounts(ctx, t, client, roleID, created.Role.Name, baseRevisionCount, baseOutboxCount)
	})

	t.Run("revision insert failure", func(t *testing.T) {
		_, err := store.Update(ctx, roleapplication.UpdateRoleInput{RoleID: roleID, Name: "Revision Failed", Active: true}, roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKindPolicyChanged, Reason: strings.Repeat("r", 65), RoleID: roleID})
		require.ErrorContains(t, err, "append rbac policy revision")
		assertRoleAndFactCounts(ctx, t, client, roleID, created.Role.Name, baseRevisionCount, baseOutboxCount)
	})

	t.Run("outbox insert failure", func(t *testing.T) {
		_, err := store.Update(ctx, roleapplication.UpdateRoleInput{RoleID: roleID, Name: "Outbox Failed", Active: true}, roleapplication.PolicyChange{Kind: roleapplication.PolicyChangeKind("invalid"), Reason: "role_updated", RoleID: roleID})
		require.ErrorContains(t, err, "append rbac policy outbox event")
		assertRoleAndFactCounts(ctx, t, client, roleID, created.Role.Name, baseRevisionCount, baseOutboxCount)
	})
}

func TestPolicyFactCommitFailureRollsBackAllWrites(t *testing.T) {
	client := newRoleStoreTestClient(t)
	ctx := context.Background()
	store := NewRoleStore(client)
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000021002")
	created, err := store.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Before Commit Failure", Active: true}, rolePolicyChange("role_created", roleID))
	require.NoError(t, err)
	baseRevisionCount, err := client.RbacPolicyRevision.Query().Count(ctx)
	require.NoError(t, err)
	baseOutboxCount, err := client.RbacPolicyOutboxEvent.Query().Count(ctx)
	require.NoError(t, err)

	commitCtx, cancel := context.WithCancel(ctx)
	client.RbacPolicyOutboxEvent.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			value, err := next.Mutate(ctx, mutation)
			if err == nil {
				cancel()
			}
			return value, err
		})
	})

	_, err = store.Update(commitCtx, roleapplication.UpdateRoleInput{RoleID: roleID, Name: "Commit Failed", Active: true}, rolePolicyChange("role_updated", roleID))
	require.ErrorIs(t, err, context.Canceled)
	assertRoleAndFactCounts(ctx, t, client, roleID, created.Role.Name, baseRevisionCount, baseOutboxCount)
}

func assertPolicyFact(ctx context.Context, t *testing.T, client *ent.Client, revision int64, change roleapplication.PolicyChange) {
	t.Helper()
	require.Positive(t, revision)
	revisionRow, err := client.RbacPolicyRevision.Query().Where(entrbacpolicyrevision.IDEQ(revision)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, change.Reason, revisionRow.Reason)
	require.Equal(t, optionalUUID(change.RoleID), revisionRow.RoleID)
	require.Equal(t, optionalUUID(change.UserID), revisionRow.UserID)
	require.Equal(t, optionalUUID(change.PermissionID), revisionRow.PermissionID)
	event, err := client.RbacPolicyOutboxEvent.Query().Where(entrbacpolicyoutboxevent.RevisionEQ(revision)).Only(ctx)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, event.EventID)
	require.Equal(t, string(change.Kind), event.Kind)
	require.Equal(t, change.Reason, event.Reason)
	require.Equal(t, optionalUUID(change.RoleID), event.RoleID)
	require.Equal(t, optionalUUID(change.UserID), event.UserID)
	require.Equal(t, optionalUUID(change.PermissionID), event.PermissionID)
	require.Equal(t, "pending", event.Status)
	require.Zero(t, event.AttemptCount)
	require.Equal(t, "rbac-policy-revision:"+strconv.FormatInt(revision, 10), event.IdempotencyKey)
}

func assertRoleAndFactCounts(ctx context.Context, t *testing.T, client *ent.Client, roleID uuid.UUID, name string, revisionCount int, outboxCount int) {
	t.Helper()
	role, err := client.Role.Query().Where(entrole.RoleIDEQ(roleID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, name, role.Name)
	gotRevisionCount, err := client.RbacPolicyRevision.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, revisionCount, gotRevisionCount)
	gotOutboxCount, err := client.RbacPolicyOutboxEvent.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, outboxCount, gotOutboxCount)
}

func optionalUUID(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	return &value
}
