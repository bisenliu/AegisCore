package catalog

const (
	// SuperAdminRoleID 是内置超级管理员角色的稳定外部 ID。
	SuperAdminRoleID = "00000000-0000-0000-0000-000000000001"
)

const (
	permissionUserList   = "00000000-0000-0000-0000-000000010001"
	permissionUserCreate = "00000000-0000-0000-0000-000000010002"
	permissionUserGet    = "00000000-0000-0000-0000-000000010003"
)

// RoleSpec 描述人工维护的系统角色基线。
type RoleSpec struct {
	RoleID      string
	Name        string
	Description string
	System      bool
}

// RolePermissionSpec 描述人工维护的系统角色默认权限绑定。
type RolePermissionSpec struct {
	RoleID       string
	PermissionID string
}

// DefaultRoles 返回用户服务当前人工维护的系统角色基线。
func DefaultRoles() []RoleSpec {
	return []RoleSpec{
		{
			RoleID:      SuperAdminRoleID,
			Name:        "超级管理员",
			Description: "拥有系统全部接口权限",
			System:      true,
		},
	}
}

// DefaultRolePermissions 返回系统角色默认权限绑定基线。
func DefaultRolePermissions() []RolePermissionSpec {
	return []RolePermissionSpec{
		{RoleID: SuperAdminRoleID, PermissionID: permissionUserList},
		{RoleID: SuperAdminRoleID, PermissionID: permissionUserCreate},
		{RoleID: SuperAdminRoleID, PermissionID: permissionUserGet},
	}
}
