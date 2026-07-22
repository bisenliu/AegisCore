package rbacbaseline

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSystemIDsMatchV5Names(t *testing.T) {
	namespace := uuid.MustParse(SystemIDNamespace)
	seen := make(map[string]string, len(systemIDCases()))

	for _, tc := range systemIDCases() {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := uuid.Parse(tc.id)
			require.NoError(t, err)
			require.Equal(t, uuid.Version(5), parsed.Version())
			require.Equal(t, uuid.NewSHA1(namespace, []byte(tc.name)).String(), tc.id)
		})

		if existing, ok := seen[tc.id]; ok {
			require.Failf(t, "duplicate system id", "%s and %s share %s", existing, tc.name, tc.id)
		}
		seen[tc.id] = tc.name
	}
}

func TestDefaultPermissionsUseRegisteredSystemIDs(t *testing.T) {
	permissionIDs := registeredPermissionIDs()
	seen := make(map[string]struct{}, len(permissionIDs))

	for _, permission := range DefaultPermissions() {
		require.Contains(t, permissionIDs, permission.PermissionID, "permission %s uses unregistered id", permission.Name)
		require.NotContains(t, seen, permission.PermissionID, "duplicate permission id")
		seen[permission.PermissionID] = struct{}{}
	}
	require.Len(t, seen, len(permissionIDs))
}

func TestDefaultRolePermissionsUseRegisteredSystemIDs(t *testing.T) {
	roleIDs := map[string]struct{}{SuperAdminRoleID: {}}
	permissionIDs := registeredPermissionIDs()

	for _, binding := range DefaultRolePermissions() {
		require.Contains(t, roleIDs, binding.RoleID, "binding references unknown role_id")
		require.Contains(t, permissionIDs, binding.PermissionID, "binding references unknown permission_id")
	}
}

type systemIDCase struct {
	name string
	id   string
}

func systemIDCases() []systemIDCase {
	return []systemIDCase{
		{name: "role:super-admin", id: SuperAdminRoleID},
		{name: "user:bootstrap-super-admin", id: BootstrapSuperAdminUserID},
		{name: "permission:user:list", id: PermissionUserListID},
		{name: "permission:user:create", id: PermissionUserCreateID},
		{name: "permission:user:get", id: PermissionUserGetID},
		{name: "permission:permission:list", id: PermissionPermissionListID},
		{name: "permission:permission:effective-by-user", id: PermissionPermissionUserEffectiveID},
		{name: "permission:role:list", id: PermissionRoleListID},
		{name: "permission:role:create", id: PermissionRoleCreateID},
		{name: "permission:role:get", id: PermissionRoleGetID},
		{name: "permission:role:update", id: PermissionRoleUpdateID},
		{name: "permission:role:set-status", id: PermissionRoleStatusID},
		{name: "permission:user-role:list", id: PermissionUserRoleListID},
		{name: "permission:user-role:replace", id: PermissionUserRoleReplaceID},
		{name: "permission:user-role:add", id: PermissionUserRoleAddID},
		{name: "permission:user-role:remove", id: PermissionUserRoleRemoveID},
		{name: "permission:role-permission:list", id: PermissionRolePermissionListID},
		{name: "permission:role-permission:replace", id: PermissionRolePermissionReplaceID},
		{name: "permission:role-permission:add", id: PermissionRolePermissionAddID},
		{name: "permission:role-permission:remove", id: PermissionRolePermissionRemoveID},
	}
}

func registeredPermissionIDs() map[string]struct{} {
	return map[string]struct{}{
		PermissionUserListID:                {},
		PermissionUserCreateID:              {},
		PermissionUserGetID:                 {},
		PermissionPermissionListID:          {},
		PermissionPermissionUserEffectiveID: {},
		PermissionRoleListID:                {},
		PermissionRoleCreateID:              {},
		PermissionRoleGetID:                 {},
		PermissionRoleUpdateID:              {},
		PermissionRoleStatusID:              {},
		PermissionUserRoleListID:            {},
		PermissionUserRoleReplaceID:         {},
		PermissionUserRoleAddID:             {},
		PermissionUserRoleRemoveID:          {},
		PermissionRolePermissionListID:      {},
		PermissionRolePermissionReplaceID:   {},
		PermissionRolePermissionAddID:       {},
		PermissionRolePermissionRemoveID:    {},
	}
}
