package rbacbaseline

const (
	// SuperAdminRoleID 是内置超级管理员角色的稳定外部 ID。
	SuperAdminRoleID = "00000000-0000-0000-0000-000000000001"
)

const (
	permissionUserList   = "00000000-0000-0000-0000-000000010001"
	permissionUserCreate = "00000000-0000-0000-0000-000000010002"
	permissionUserGet    = "00000000-0000-0000-0000-000000010003"
)

// RoleSpec 描述系统 RBAC 角色基线。
type RoleSpec struct {
	RoleID      string
	Name        string
	Description string
	System      bool
}

// PermissionSpec 描述系统 RBAC 权限基线。
type PermissionSpec struct {
	PermissionID string
	Name         string
	Description  string
	Module       string
	Method       string
	PathTemplate string
	System       bool
}

// RolePermissionSpec 描述系统 RBAC 角色权限绑定基线。
type RolePermissionSpec struct {
	RoleID       string
	PermissionID string
}

// DefaultRoles 返回用户服务当前系统 RBAC 角色基线。
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

// DefaultPermissions 返回用户服务当前系统 RBAC 权限基线。
func DefaultPermissions() []PermissionSpec {
	return []PermissionSpec{
		{
			PermissionID: permissionUserList,
			Name:         "查询用户列表",
			Description:  "允许分页查询用户资料",
			Module:       "user",
			Method:       "GET",
			PathTemplate: "/api/v1/users",
			System:       true,
		},
		{
			PermissionID: permissionUserCreate,
			Name:         "创建用户",
			Description:  "允许创建用户资料",
			Module:       "user",
			Method:       "POST",
			PathTemplate: "/api/v1/users",
			System:       true,
		},
		{
			PermissionID: permissionUserGet,
			Name:         "查看用户详情",
			Description:  "允许按用户 ID 查询用户资料",
			Module:       "user",
			Method:       "GET",
			PathTemplate: "/api/v1/users/:user_id",
			System:       true,
		},
	}
}

// DefaultRolePermissions 返回系统 RBAC 角色权限绑定基线。
func DefaultRolePermissions() []RolePermissionSpec {
	return []RolePermissionSpec{
		{RoleID: SuperAdminRoleID, PermissionID: permissionUserList},
		{RoleID: SuperAdminRoleID, PermissionID: permissionUserCreate},
		{RoleID: SuperAdminRoleID, PermissionID: permissionUserGet},
	}
}
