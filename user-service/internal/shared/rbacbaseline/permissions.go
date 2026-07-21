package rbacbaseline

const (
	permissionUserList   = "00000000-0000-0000-0000-000000010001"
	permissionUserCreate = "00000000-0000-0000-0000-000000010002"
	permissionUserGet    = "00000000-0000-0000-0000-000000010003"

	permissionPermissionList          = "00000000-0000-0000-0000-000000020001"
	permissionPermissionUserEffective = "00000000-0000-0000-0000-000000020002"

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
		},
		{
			PermissionID: permissionUserCreate,
			Name:         "创建用户",
			Description:  "允许创建用户资料",
			Module:       "user",
			Method:       "POST",
			PathTemplate: "/api/v1/users",
		},
		{
			PermissionID: permissionUserGet,
			Name:         "查看用户详情",
			Description:  "允许按用户 ID 查询用户资料",
			Module:       "user",
			Method:       "GET",
			PathTemplate: "/api/v1/users/:user_id",
		},
		{
			PermissionID: permissionPermissionList,
			Name:         "查询权限列表",
			Description:  "允许分页查询权限目录",
			Module:       "permission",
			Method:       "GET",
			PathTemplate: "/api/v1/permissions",
		},
		{
			PermissionID: permissionPermissionUserEffective,
			Name:         "查询用户有效权限",
			Description:  "允许查询用户经角色绑定后当前生效的权限集合",
			Module:       "permission",
			Method:       "GET",
			PathTemplate: "/api/v1/permissions/users/:user_id/effective",
		},
		{
			PermissionID: permissionRoleList,
			Name:         "查询角色列表",
			Description:  "允许分页查询角色",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/roles",
		},
		{
			PermissionID: permissionRoleCreate,
			Name:         "创建角色",
			Description:  "允许创建角色",
			Module:       "role",
			Method:       "POST",
			PathTemplate: "/api/v1/roles",
		},
		{
			PermissionID: permissionRoleGet,
			Name:         "查询角色详情",
			Description:  "允许按角色 ID 查询角色详情",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/roles/:role_id",
		},
		{
			PermissionID: permissionRoleUpdate,
			Name:         "更新角色",
			Description:  "允许更新角色元数据",
			Module:       "role",
			Method:       "PATCH",
			PathTemplate: "/api/v1/roles/:role_id",
		},
		{
			PermissionID: permissionRoleStatus,
			Name:         "启停角色",
			Description:  "允许启用或停用角色",
			Module:       "role",
			Method:       "PATCH",
			PathTemplate: "/api/v1/roles/:role_id/status",
		},
		{
			PermissionID: permissionUserRoleList,
			Name:         "查询用户角色",
			Description:  "允许查询用户当前绑定的角色列表",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/users/:user_id/roles",
		},
		{
			PermissionID: permissionUserRoleReplace,
			Name:         "替换用户角色",
			Description:  "允许幂等替换用户完整角色集合",
			Module:       "role",
			Method:       "PUT",
			PathTemplate: "/api/v1/users/:user_id/roles",
		},
		{
			PermissionID: permissionUserRoleAdd,
			Name:         "绑定用户角色",
			Description:  "允许为用户新增一个角色绑定",
			Module:       "role",
			Method:       "POST",
			PathTemplate: "/api/v1/users/:user_id/roles",
		},
		{
			PermissionID: permissionUserRoleRemove,
			Name:         "解绑用户角色",
			Description:  "允许删除用户角色绑定",
			Module:       "role",
			Method:       "DELETE",
			PathTemplate: "/api/v1/users/:user_id/roles/:role_id",
		},
		{
			PermissionID: permissionRolePermissionList,
			Name:         "查询角色权限",
			Description:  "允许查询角色绑定的权限列表",
			Module:       "role",
			Method:       "GET",
			PathTemplate: "/api/v1/roles/:role_id/permissions",
		},
		{
			PermissionID: permissionRolePermissionReplace,
			Name:         "替换角色权限",
			Description:  "允许幂等替换角色完整权限集合",
			Module:       "role",
			Method:       "PUT",
			PathTemplate: "/api/v1/roles/:role_id/permissions",
		},
		{
			PermissionID: permissionRolePermissionAdd,
			Name:         "绑定角色权限",
			Description:  "允许为角色新增一个启用权限绑定",
			Module:       "role",
			Method:       "POST",
			PathTemplate: "/api/v1/roles/:role_id/permissions",
		},
		{
			PermissionID: permissionRolePermissionRemove,
			Name:         "解绑角色权限",
			Description:  "允许删除角色权限绑定",
			Module:       "role",
			Method:       "DELETE",
			PathTemplate: "/api/v1/roles/:role_id/permissions/:permission_id",
		},
	}
}
