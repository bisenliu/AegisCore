package rbacbaseline

const (
	// SuperAdminRoleID 是内置超级管理员角色的稳定外部 ID。
	SuperAdminRoleID = "00000000-0000-0000-0000-000000000001"
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

// DefaultRolePermissions 返回系统 RBAC 角色权限绑定基线。
// 超级管理员绑定当前 DefaultPermissions 的完整集合；新增系统权限会自动进入基线，是否删除旧绑定由 seed 的 SyncSystemBindings 决定。
func DefaultRolePermissions() []RolePermissionSpec {
	permissions := DefaultPermissions()
	bindings := make([]RolePermissionSpec, 0, len(permissions))
	for _, permission := range permissions {
		bindings = append(bindings, RolePermissionSpec{RoleID: SuperAdminRoleID, PermissionID: permission.PermissionID})
	}
	return bindings
}
