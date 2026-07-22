package rbacbaseline

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var reservedSystemIDPattern = regexp.MustCompile(`^00000000-0000-0000-0000-[0-9]{12}$`)

func TestSystemIDsUseReservedFormat(t *testing.T) {
	for _, tc := range systemIDCases() {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uuid.Parse(tc.id)
			require.NoError(t, err)
			require.Regexp(t, reservedSystemIDPattern, tc.id)
		})
	}
}

func TestSystemIDsMatchTypeModule(t *testing.T) {
	for _, tc := range systemIDCases() {
		t.Run(tc.name, func(t *testing.T) {
			suffix := systemIDSuffix(tc.id)
			require.Equal(t, tc.typeCode, suffix[:2])
			require.Equal(t, tc.module, suffix[2:4])
			require.NotEqual(t, "00000000", suffix[4:])
		})
	}
}

func TestSystemIDsGloballyUnique(t *testing.T) {
	seen := make(map[string]string, len(systemIDCases()))
	for _, tc := range systemIDCases() {
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
	name     string
	id       string
	typeCode string
	module   string
}

func systemIDCases() []systemIDCase {
	return []systemIDCase{
		{name: "user:bootstrap-super-admin", id: BootstrapSuperAdminUserID, typeCode: "01", module: "00"},
		{name: "role:super-admin", id: SuperAdminRoleID, typeCode: "02", module: "00"},

		{name: "permission:user:list", id: PermissionUserListID, typeCode: "03", module: "01"},
		{name: "permission:user:create", id: PermissionUserCreateID, typeCode: "03", module: "01"},
		{name: "permission:user:get", id: PermissionUserGetID, typeCode: "03", module: "01"},

		{name: "permission:permission:list", id: PermissionPermissionListID, typeCode: "03", module: "02"},
		{name: "permission:permission:effective-by-user", id: PermissionPermissionUserEffectiveID, typeCode: "03", module: "02"},

		{name: "permission:role:list", id: PermissionRoleListID, typeCode: "03", module: "03"},
		{name: "permission:role:create", id: PermissionRoleCreateID, typeCode: "03", module: "03"},
		{name: "permission:role:get", id: PermissionRoleGetID, typeCode: "03", module: "03"},
		{name: "permission:role:update", id: PermissionRoleUpdateID, typeCode: "03", module: "03"},
		{name: "permission:role:set-status", id: PermissionRoleStatusID, typeCode: "03", module: "03"},

		{name: "permission:user-role:list", id: PermissionUserRoleListID, typeCode: "03", module: "04"},
		{name: "permission:user-role:replace", id: PermissionUserRoleReplaceID, typeCode: "03", module: "04"},
		{name: "permission:user-role:add", id: PermissionUserRoleAddID, typeCode: "03", module: "04"},
		{name: "permission:user-role:remove", id: PermissionUserRoleRemoveID, typeCode: "03", module: "04"},

		{name: "permission:role-permission:list", id: PermissionRolePermissionListID, typeCode: "03", module: "05"},
		{name: "permission:role-permission:replace", id: PermissionRolePermissionReplaceID, typeCode: "03", module: "05"},
		{name: "permission:role-permission:add", id: PermissionRolePermissionAddID, typeCode: "03", module: "05"},
		{name: "permission:role-permission:remove", id: PermissionRolePermissionRemoveID, typeCode: "03", module: "05"},
	}
}

func systemIDSuffix(id string) string {
	parts := strings.Split(id, "-")
	return parts[len(parts)-1]
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
