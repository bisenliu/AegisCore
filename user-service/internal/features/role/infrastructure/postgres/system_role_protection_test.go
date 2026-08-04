package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

func TestSystemRoleOrdinaryWritesAreProtectedWithoutPolicyFacts(t *testing.T) {
	ctx, _, client := newBootstrapPostgresTestDB(t)
	roles := NewRoleStore(client)
	bindings := NewRolePermissionStore(client)
	permissions := permissionpostgres.NewPermissionStore(client)
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000022001")
	boundPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000022002")
	otherPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000022003")

	role, _, err := roles.UpsertSystemRole(ctx, roleapplication.SeedRoleInput{
		RoleID: roleID, Name: "System Role", Description: "trusted description", Active: true,
	})
	require.NoError(t, err)
	boundPermission := createPermissionForTest(ctx, t, permissions, boundPermissionID, "Bound Permission", "GET", "/api/v1/system-role/bound", true)
	otherPermission := createPermissionForTest(ctx, t, permissions, otherPermissionID, "Other Permission", "POST", "/api/v1/system-role/other", true)
	_, err = bindings.EnsureSystemBindings(ctx, roleID, []uuid.UUID{boundPermissionID})
	require.NoError(t, err)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "description only update",
			run: func() error {
				_, err := roles.Update(ctx, roleapplication.UpdateRoleInput{RoleID: roleID, Name: role.Name, Description: "tampered", Active: role.Active}, rolePolicyChange("role_updated", roleID))
				return err
			},
		},
		{
			name: "identical metadata update",
			run: func() error {
				_, err := roles.Update(ctx, roleapplication.UpdateRoleInput{RoleID: roleID, Name: role.Name, Description: role.Description, Active: role.Active}, rolePolicyChange("role_updated", roleID))
				return err
			},
		},
		{
			name: "status change",
			run: func() error {
				_, err := roles.SetActive(ctx, roleID, false, rolePolicyChange("role_active_changed", roleID))
				return err
			},
		},
		{
			name: "identical status",
			run: func() error {
				_, err := roles.SetActive(ctx, roleID, true, rolePolicyChange("role_active_changed", roleID))
				return err
			},
		},
		{
			name: "add permission",
			run: func() error {
				_, err := bindings.Add(ctx, roleID, otherPermission, permissionPolicyChange("role_permission_added", roleID, otherPermissionID))
				return err
			},
		},
		{
			name: "replace permissions",
			run: func() error {
				_, err := bindings.Replace(ctx, roleID, []roleapplication.PermissionReference{otherPermission}, permissionPolicyChange("role_permissions_replaced", roleID, uuid.Nil))
				return err
			},
		},
		{
			name: "remove permission",
			run: func() error {
				_, err := bindings.Remove(ctx, roleID, boundPermission.PermissionID, permissionPolicyChange("role_permission_removed", roleID, boundPermissionID))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshotProtectedRoleState(ctx, t, client, bindings, roleID)
			err := tt.run()
			require.ErrorIs(t, err, roledomain.ErrSystemRoleProtected)
			after := snapshotProtectedRoleState(ctx, t, client, bindings, roleID)
			require.Equal(t, before, after)
		})
	}
}

func TestSystemRoleProtectionWaitsForConcurrentSeedCommit(t *testing.T) {
	ctx, db, client := newBootstrapPostgresTestDB(t)
	roles := NewRoleStore(client)
	bindings := NewRolePermissionStore(client)
	permissions := permissionpostgres.NewPermissionStore(client)
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000023001")
	permission := createPermissionForTest(ctx, t, permissions, permissionID, "Concurrent Permission", "GET", "/api/v1/concurrent", true)

	tests := []struct {
		name   string
		roleID uuid.UUID
		run    func(uuid.UUID) error
	}{
		{
			name:   "metadata update",
			roleID: uuid.MustParse("018f0000-0000-7000-8000-000000023002"),
			run: func(roleID uuid.UUID) error {
				_, err := roles.Update(ctx, roleapplication.UpdateRoleInput{RoleID: roleID, Name: "Public Name", Description: "public", Active: false}, rolePolicyChange("role_updated", roleID))
				return err
			},
		},
		{
			name:   "status update",
			roleID: uuid.MustParse("018f0000-0000-7000-8000-000000023003"),
			run: func(roleID uuid.UUID) error {
				_, err := roles.SetActive(ctx, roleID, false, rolePolicyChange("role_active_changed", roleID))
				return err
			},
		},
		{
			name:   "permission binding",
			roleID: uuid.MustParse("018f0000-0000-7000-8000-000000023004"),
			run: func(roleID uuid.UUID) error {
				_, err := bindings.Add(ctx, roleID, permission, permissionPolicyChange("role_permission_added", roleID, permissionID))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createRoleForTest(ctx, t, roles, tt.roleID, "Ordinary Before Seed", true, false)
			beforeFacts := snapshotPolicyFacts(ctx, t, client)

			seedTx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = seedTx.Rollback() })
			_, err = seedTx.ExecContext(ctx, `UPDATE roles SET name = $1, description = $2, active = true, is_system = true WHERE role_id = $3`, "Seed System Role", "seed trusted", tt.roleID)
			require.NoError(t, err)

			result := make(chan error, 1)
			go func() { result <- tt.run(tt.roleID) }()

			select {
			case err := <-result:
				require.Failf(t, "ordinary write did not wait for seed lock", "returned early with %v", err)
			case <-time.After(150 * time.Millisecond):
			}

			require.NoError(t, seedTx.Commit())
			select {
			case err := <-result:
				require.ErrorIs(t, err, roledomain.ErrSystemRoleProtected)
			case <-time.After(5 * time.Second):
				require.Fail(t, "ordinary write did not finish after seed commit")
			}

			got, err := roles.GetByRoleID(ctx, tt.roleID)
			require.NoError(t, err)
			require.Equal(t, "Seed System Role", got.Name)
			require.Equal(t, "seed trusted", got.Description)
			require.True(t, got.Active)
			require.True(t, got.IsSystem)
			require.Empty(t, permissionIDsForTest(mustListRolePermissions(ctx, t, bindings, tt.roleID)))
			require.Equal(t, beforeFacts, snapshotPolicyFacts(ctx, t, client))
		})
	}
}

type protectedRoleState struct {
	Role          roledomain.Role
	PermissionIDs []uuid.UUID
	PolicyFacts   policyFactState
}

type policyFactState struct {
	CounterExists bool
	Counter       int64
	Revisions     int
	OutboxEvents  int
}

func snapshotProtectedRoleState(ctx context.Context, t *testing.T, client *ent.Client, bindings *RolePermissionStore, roleID uuid.UUID) protectedRoleState {
	t.Helper()
	found, err := NewRoleStore(client).GetByRoleID(ctx, roleID)
	require.NoError(t, err)
	return protectedRoleState{
		Role:          *found,
		PermissionIDs: permissionIDsForTest(mustListRolePermissions(ctx, t, bindings, roleID)),
		PolicyFacts:   snapshotPolicyFacts(ctx, t, client),
	}
}

func snapshotPolicyFacts(ctx context.Context, t *testing.T, client *ent.Client) policyFactState {
	t.Helper()
	state := policyFactState{}
	counter, err := client.RbacPolicyRevisionCounter.Get(ctx, policyRevisionCounterID)
	if err == nil {
		state.CounterExists = true
		state.Counter = counter.LastRevision
	} else {
		require.True(t, ent.IsNotFound(err), "read policy revision counter: %v", err)
	}
	state.Revisions, err = client.RbacPolicyRevision.Query().Count(ctx)
	require.NoError(t, err)
	state.OutboxEvents, err = client.RbacPolicyOutboxEvent.Query().Count(ctx)
	require.NoError(t, err)
	return state
}

func mustListRolePermissions(ctx context.Context, t *testing.T, store *RolePermissionStore, roleID uuid.UUID) []roleapplication.PermissionReference {
	t.Helper()
	items, err := store.ListByRoleID(ctx, roleID)
	require.NoError(t, err)
	return items
}
