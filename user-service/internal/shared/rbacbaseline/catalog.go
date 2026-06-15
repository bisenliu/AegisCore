package rbacbaseline

const (
	// SuperAdminRoleID 是内置超级管理员角色的稳定外部 ID。
	SuperAdminRoleID = "00000000-0000-0000-0000-000000000001"
)

const (
	permissionUserList   = "00000000-0000-0000-0000-000000010001"
	permissionUserCreate = "00000000-0000-0000-0000-000000010002"
	permissionUserGet    = "00000000-0000-0000-0000-000000010003"

	permissionPermissionList          = "00000000-0000-0000-0000-000000020001"
	permissionPermissionCreate        = "00000000-0000-0000-0000-000000020002"
	permissionPermissionRouteDiff     = "00000000-0000-0000-0000-000000020003"
	permissionPermissionUserEffective = "00000000-0000-0000-0000-000000020004"
	permissionPermissionGet           = "00000000-0000-0000-0000-000000020005"
	permissionPermissionUpdate        = "00000000-0000-0000-0000-000000020006"
	permissionPermissionEnable        = "00000000-0000-0000-0000-000000020007"
	permissionPermissionDisable       = "00000000-0000-0000-0000-000000020008"

	permissionRoleList   = "00000000-0000-0000-0000-000000030001"
	permissionRoleCreate = "00000000-0000-0000-0000-000000030002"
	permissionRoleGet    = "00000000-0000-0000-0000-000000030003"
	permissionRoleUpdate = "00000000-0000-0000-0000-000000030004"
	permissionRoleStatus = "00000000-0000-0000-0000-000000030005"

	permissionUserRoleList    = "00000000-0000-0000-0000-000000040001"
	permissionUserRoleReplace = "00000000-0000-0000-0000-000000040002"
	permissionUserRoleAdd     = "00000000-0000-0000-0000-000000040003"
	permissionUserRoleRemove  = "00000000-0000-0000-0000-000000040004"

	permissionRolePermissionList    = "00000000-0000-0000-0000-000000050001"
	permissionRolePermissionReplace = "00000000-0000-0000-0000-000000050002"
	permissionRolePermissionAdd     = "00000000-0000-0000-0000-000000050003"
	permissionRolePermissionRemove  = "00000000-0000-0000-0000-000000050004"
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
		{
			PermissionID: permissionPermissionList,
			Name:         "查询权限列表",
			Description:  "允许分页查询权限目录",
			Module:       "permission",
			Method:       "GET",
			PathTemplate: "/api/v1/permissions",
			System:       true,
		},
		{
			PermissionID: permissionPermissionCreate,
			Name:         "创建权限",
			Description:  "允许创建正式权限目录记录",
			Module:       "permission",
			Method:       "POST",
			PathTemplate: "/api/v1/permissions",
			System:       true,
		},
		{
			PermissionID: permissionPermissionRouteDiff,
			Name:         "查询权限路由差异",
			Description:  "允许只读查询已注册路由与权限目录差异",
			Module:       "permission",
			Method:       "GET",
			PathTemplate: "/api/v1/permissions/route-diff",
			System:       true,
		},
		{
			PermissionID: permissionPermissionUserEffective,
			Name:         "查询用户有效权限",
			Description:  "允许查询用户经角色绑定后当前生效的权限集合",
			Module:       "permission",
			Method:       "GET",
			PathTemplate: "/api/v1/permissions/users/:user_id/effective",
			System:       true,
		},
		{
			PermissionID: permissionPermissionGet,
			Name:         "查询权限详情",
			Description:  "允许按权限 ID 查询权限目录记录",
			Module:       "permission",
			Method:       "GET",
			PathTemplate: "/api/v1/permissions/:permission_id",
			System:       true,
		},
		{
			PermissionID: permissionPermissionUpdate,
			Name:         "更新权限",
			Description:  "允许更新权限目录记录",
			Module:       "permission",
			Method:       "PUT",
			PathTemplate: "/api/v1/permissions/:permission_id",
			System:       true,
		},
		{
			PermissionID: permissionPermissionEnable,
			Name:         "启用权限",
			Description:  "允许启用权限目录记录",
			Module:       "permission",
			Method:       "POST",
			PathTemplate: "/api/v1/permissions/:permission_id/enable",
			System:       true,
		},
		{
			PermissionID: permissionPermissionDisable,
			Name:         "停用权限",
			Description:  "允许停用权限目录记录",
			Module:       "permission",
			Method:       "POST",
			PathTemplate: "/api/v1/permissions/:permission_id/disable",
			System:       true,
		},
		{
			PermissionID: permissionRoleList,
			Name:         "查询角色列表",
			Description:  "允许分页查询角色",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/roles",
			System:       true,
		},
		{
			PermissionID: permissionRoleCreate,
			Name:         "创建角色",
			Description:  "允许创建角色",
			Module:       "role",
			Method:       "POST",
			PathTemplate: "/api/v1/roles",
			System:       true,
		},
		{
			PermissionID: permissionRoleGet,
			Name:         "查询角色详情",
			Description:  "允许按角色 ID 查询角色详情",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/roles/:role_id",
			System:       true,
		},
		{
			PermissionID: permissionRoleUpdate,
			Name:         "更新角色",
			Description:  "允许更新角色元数据",
			Module:       "role",
			Method:       "PATCH",
			PathTemplate: "/api/v1/roles/:role_id",
			System:       true,
		},
		{
			PermissionID: permissionRoleStatus,
			Name:         "启停角色",
			Description:  "允许启用或停用角色",
			Module:       "role",
			Method:       "PATCH",
			PathTemplate: "/api/v1/roles/:role_id/status",
			System:       true,
		},
		{
			PermissionID: permissionUserRoleList,
			Name:         "查询用户角色",
			Description:  "允许查询用户当前绑定的角色列表",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/users/:user_id/roles",
			System:       true,
		},
		{
			PermissionID: permissionUserRoleReplace,
			Name:         "替换用户角色",
			Description:  "允许幂等替换用户完整角色集合",
			Module:       "role",
			Method:       "PUT",
			PathTemplate: "/api/v1/users/:user_id/roles",
			System:       true,
		},
		{
			PermissionID: permissionUserRoleAdd,
			Name:         "绑定用户角色",
			Description:  "允许为用户新增一个角色绑定",
			Module:       "role",
			Method:       "POST",
			PathTemplate: "/api/v1/users/:user_id/roles",
			System:       true,
		},
		{
			PermissionID: permissionUserRoleRemove,
			Name:         "解绑用户角色",
			Description:  "允许删除用户角色绑定",
			Module:       "role",
			Method:       "DELETE",
			PathTemplate: "/api/v1/users/:user_id/roles/:role_id",
			System:       true,
		},
		{
			PermissionID: permissionRolePermissionList,
			Name:         "查询角色权限",
			Description:  "允许查询角色绑定的权限列表",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/roles/:role_id/permissions",
			System:       true,
		},
		{
			PermissionID: permissionRolePermissionReplace,
			Name:         "替换角色权限",
			Description:  "允许幂等替换角色完整权限集合",
			Module:       "role",
			Method:       "PUT",
			PathTemplate: "/api/v1/roles/:role_id/permissions",
			System:       true,
		},
		{
			PermissionID: permissionRolePermissionAdd,
			Name:         "绑定角色权限",
			Description:  "允许为角色新增一个启用权限绑定",
			Module:       "role",
			Method:       "POST",
			PathTemplate: "/api/v1/roles/:role_id/permissions",
			System:       true,
		},
		{
			PermissionID: permissionRolePermissionRemove,
			Name:         "解绑角色权限",
			Description:  "允许删除角色权限绑定",
			Module:       "role",
			Method:       "DELETE",
			PathTemplate: "/api/v1/roles/:role_id/permissions/:permission_id",
			System:       true,
		},
	}
}

// DefaultRolePermissions 返回系统 RBAC 角色权限绑定基线。
func DefaultRolePermissions() []RolePermissionSpec {
	permissions := DefaultPermissions()
	bindings := make([]RolePermissionSpec, 0, len(permissions))
	for _, permission := range permissions {
		bindings = append(bindings, RolePermissionSpec{RoleID: SuperAdminRoleID, PermissionID: permission.PermissionID})
	}
	return bindings
}
