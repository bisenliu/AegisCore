package rbacbaseline

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
}

// RolePermissionSpec 描述系统 RBAC 角色权限绑定基线。
type RolePermissionSpec struct {
	RoleID       string
	PermissionID string
}

type defaultRoleSpec struct {
	RoleSpec
	PermissionIDs func() []string
}

var defaultRoleCatalog = []defaultRoleSpec{
	{
		RoleSpec: RoleSpec{
			RoleID:      SuperAdminRoleID,
			Name:        "超级管理员",
			Description: "拥有系统全部接口权限",
			System:      true,
		},
		PermissionIDs: allPermissionIDs,
	},
	// 未来新增默认角色时，在这里新增一个 role block，并显式列出权限 ID。
	// 不要按 Module、model 或 read/write 粗粒度自动推导权限，避免误授权。
	//
	// 示例：
	// {
	// 	RoleSpec: RoleSpec{
	// 		RoleID:      ExampleRoleID,
	// 		Name:        "示例角色",
	// 		Description: "只能查看和创建用户，不能修改用户",
	// 		System:      true,
	// 	},
	// 	PermissionIDs: permissionIDs(
	// 		PermissionUserListID,
	// 		PermissionUserGetID,
	// 		PermissionUserCreateID,
	// 	),
	// },
}

// DefaultRoles 返回用户服务当前系统 RBAC 角色基线。
func DefaultRoles() []RoleSpec {
	roles := make([]RoleSpec, 0, len(defaultRoleCatalog))
	for _, role := range defaultRoleCatalog {
		roles = append(roles, role.RoleSpec)
	}
	return roles
}

// DefaultRolePermissions 返回系统 RBAC 角色权限绑定基线。
// 超级管理员绑定当前 DefaultPermissions 的完整集合；新增系统权限会自动进入基线，是否删除旧绑定由 seed 的 SyncSystemBindings 决定。
func DefaultRolePermissions() []RolePermissionSpec {
	var bindings []RolePermissionSpec
	for _, role := range defaultRoleCatalog {
		for _, permissionID := range role.PermissionIDs() {
			bindings = append(bindings, RolePermissionSpec{
				RoleID:       role.RoleID,
				PermissionID: permissionID,
			})
		}
	}
	return bindings
}

func allPermissionIDs() []string {
	permissions := DefaultPermissions()
	ids := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		ids = append(ids, permission.PermissionID)
	}
	return ids
}
