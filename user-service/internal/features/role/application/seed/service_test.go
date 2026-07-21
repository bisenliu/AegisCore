package seed

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestSeedServiceDefaultEnsureAndRepeat(t *testing.T) {
	ctrl := gomock.NewController(t)
	stores := newSeedMockStores(ctrl)
	service := NewService(stores.roles, stores.permissions, stores.rolePermissions, stores.userRoles)
	permissionCount := len(rbacbaseline.DefaultPermissions())

	firstCalls := append(expectRoleUpserts(stores.roles, false, true), expectPermissionUpserts(stores.permissions, false, true)...)
	firstCalls = append(firstCalls, expectEnsureSystemBindings(stores.rolePermissions, permissionCount)...)
	repeatCalls := append(expectRoleUpserts(stores.roles, false, false), expectPermissionUpserts(stores.permissions, false, false)...)
	repeatCalls = append(repeatCalls, expectEnsureSystemBindings(stores.rolePermissions, 0)...)
	inOrder(append(firstCalls, repeatCalls...))

	result, err := service.Seed(context.Background(), SeedOptions{})
	require.NoError(t, err)
	require.Equal(t, len(rbacbaseline.DefaultRoles()), result.RolesInserted)
	require.Equal(t, permissionCount, result.PermissionsInserted)
	require.Equal(t, permissionCount, result.RolePermissionBindingsAdd)

	result, err = service.Seed(context.Background(), SeedOptions{})
	require.NoError(t, err)
	require.Zero(t, result.RolesInserted)
	require.Equal(t, len(rbacbaseline.DefaultRoles()), result.RolesUpdated)
	require.Zero(t, result.PermissionsInserted)
	require.Equal(t, permissionCount, result.PermissionsUpdated)
	require.Zero(t, result.RolePermissionBindingsAdd)
}

func TestSeedServiceReactivateAndSyncOptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	stores := newSeedMockStores(ctrl)
	service := NewService(stores.roles, stores.permissions, stores.rolePermissions, stores.userRoles)
	permissionCount := len(rbacbaseline.DefaultPermissions())

	calls := append(expectRoleUpserts(stores.roles, true, true), expectPermissionUpserts(stores.permissions, true, true)...)
	calls = append(calls, expectSyncSystemBindings(stores.rolePermissions, permissionCount, 0)...)
	inOrder(calls)

	result, err := service.Seed(context.Background(), SeedOptions{ReactivateSystem: true, SyncSystemBindings: true})
	require.NoError(t, err)
	require.Equal(t, len(rbacbaseline.DefaultRoles()), result.RolesInserted)
	require.Equal(t, permissionCount, result.PermissionsInserted)
	require.Equal(t, permissionCount, result.RolePermissionBindingsAdd)
	require.Zero(t, result.RolePermissionBindingsDel)
}

func TestSeedServiceAssignSuperAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	stores := newSeedMockStores(ctrl)
	service := NewService(stores.roles, stores.permissions, stores.rolePermissions, stores.userRoles)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	superAdminRoleID := uuid.MustParse(rbacbaseline.SuperAdminRoleID)

	gomock.InOrder(
		stores.userRoles.EXPECT().AssignRole(gomock.Any(), userID, superAdminRoleID).Return(true, nil),
		stores.userRoles.EXPECT().AssignRole(gomock.Any(), userID, superAdminRoleID).Return(false, nil),
	)

	result, err := service.AssignSuperAdmin(context.Background(), userID)
	require.NoError(t, err)
	require.True(t, result.Added)

	result, err = service.AssignSuperAdmin(context.Background(), userID)
	require.NoError(t, err)
	require.False(t, result.Added)
}

type seedMockStores struct {
	permissions     *MockSeedPermissionStore
	roles           *MockSeedRoleStore
	rolePermissions *MockSeedRolePermissionStore
	userRoles       *MockSeedUserRoleStore
}

func newSeedMockStores(ctrl *gomock.Controller) seedMockStores {
	return seedMockStores{
		permissions:     NewMockSeedPermissionStore(ctrl),
		roles:           NewMockSeedRoleStore(ctrl),
		rolePermissions: NewMockSeedRolePermissionStore(ctrl),
		userRoles:       NewMockSeedUserRoleStore(ctrl),
	}
}

func inOrder(calls []*gomock.Call) {
	ordered := make([]any, 0, len(calls))
	for _, call := range calls {
		ordered = append(ordered, call)
	}
	gomock.InOrder(ordered...)
}

func expectRoleUpserts(store *MockSeedRoleStore, reactivate bool, inserted bool) []*gomock.Call {
	roles := rbacbaseline.DefaultRoles()
	calls := make([]*gomock.Call, 0, len(roles))
	for _, spec := range roles {
		input := expectedSeedRoleInput(spec, reactivate)
		role := roleFromSeedInput(input)
		calls = append(calls, store.EXPECT().UpsertSystemRole(gomock.Any(), input).Return(role, inserted, nil))
	}
	return calls
}

func expectPermissionUpserts(store *MockSeedPermissionStore, reactivate bool, inserted bool) []*gomock.Call {
	permissions := rbacbaseline.DefaultPermissions()
	calls := make([]*gomock.Call, 0, len(permissions))
	for _, spec := range permissions {
		input := expectedSeedPermissionInput(spec, reactivate)
		permission := permissionFromSeedInput(input)
		calls = append(calls, store.EXPECT().UpsertPermission(gomock.Any(), input).Return(permission, inserted, nil))
	}
	return calls
}

func expectEnsureSystemBindings(store *MockSeedRolePermissionStore, added int) []*gomock.Call {
	bindings := expectedRolePermissionInputs()
	calls := make([]*gomock.Call, 0, len(bindings))
	for roleID, permissionIDs := range bindings {
		calls = append(calls, store.EXPECT().EnsureSystemBindings(gomock.Any(), roleID, permissionIDs).Return(added, nil))
	}
	return calls
}

func expectSyncSystemBindings(store *MockSeedRolePermissionStore, added int, removed int) []*gomock.Call {
	bindings := expectedRolePermissionInputs()
	calls := make([]*gomock.Call, 0, len(bindings))
	for roleID, permissionIDs := range bindings {
		calls = append(calls, store.EXPECT().SyncSystemBindings(gomock.Any(), roleID, permissionIDs).Return(added, removed, nil))
	}
	return calls
}

func expectedSeedRoleInput(spec rbacbaseline.RoleSpec, reactivate bool) roleapplication.SeedRoleInput {
	return roleapplication.SeedRoleInput{
		RoleID:           uuid.MustParse(spec.RoleID),
		Name:             spec.Name,
		Description:      spec.Description,
		Active:           true,
		ReactivateSystem: reactivate,
	}
}

func expectedSeedPermissionInput(spec rbacbaseline.PermissionSpec, _ bool) permissionapplication.SeedPermissionInput {
	return permissionapplication.SeedPermissionInput{
		PermissionID: uuid.MustParse(spec.PermissionID),
		Name:         spec.Name,
		Description:  spec.Description,
		Module:       spec.Module,
		HTTPMethod:   spec.Method,
		PathTemplate: spec.PathTemplate,
	}
}

func expectedRolePermissionInputs() map[uuid.UUID][]uuid.UUID {
	bindings := make(map[uuid.UUID][]uuid.UUID)
	for _, spec := range rbacbaseline.DefaultRolePermissions() {
		roleID := uuid.MustParse(spec.RoleID)
		permissionID := uuid.MustParse(spec.PermissionID)
		bindings[roleID] = append(bindings[roleID], permissionID)
	}
	return bindings
}

func roleFromSeedInput(input roleapplication.SeedRoleInput) *roledomain.Role {
	return &roledomain.Role{RoleID: input.RoleID, Name: input.Name, Description: input.Description, Active: input.Active, IsSystem: true}
}

func permissionFromSeedInput(input permissionapplication.SeedPermissionInput) *permissiondomain.Permission {
	return &permissiondomain.Permission{PermissionID: input.PermissionID, Name: input.Name, Description: input.Description, Module: input.Module, HTTPMethod: input.HTTPMethod, PathTemplate: input.PathTemplate}
}
