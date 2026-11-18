package rbacbaseline

// 系统保留 ID 格式为 00000000-0000-0000-0000-TTMMSSSSSSSS。
// TT 表示类型：01=系统用户，02=系统角色，03=系统权限，09=测试、fixture、诊断预留。
// MM 表示模块：00=无模块，01=user，02=permission，03=role，04=user-role，05=role-permission。
// SSSSSSSS 是同一 TT+MM 下从 00000001 开始递增的序号；已发布或已删除的 ID 不得修改或复用。
const (
	// BootstrapSuperAdminUserID 预设的首位超级管理员用户的固定唯一编号。
	BootstrapSuperAdminUserID = "00000000-0000-0000-0000-010000000001"

	// SuperAdminRoleID 预设的超级管理员角色的固定唯一编号。
	SuperAdminRoleID = "00000000-0000-0000-0000-020000000001"

	// PermissionUserListID 查看用户列表功能的固定权限编号。
	PermissionUserListID = "00000000-0000-0000-0000-030100000001"
	// PermissionUserCreateID 创建新用户功能的固定权限编号。
	PermissionUserCreateID = "00000000-0000-0000-0000-030100000002"
	// PermissionUserGetID 查看单个用户详情功能的固定权限编号。
	PermissionUserGetID = "00000000-0000-0000-0000-030100000003"

	// PermissionPermissionListID 查看系统权限列表功能的固定权限编号。
	PermissionPermissionListID = "00000000-0000-0000-0000-030200000001"
	// PermissionPermissionUserEffectiveID 查询某位用户当前实际拥有的全部权限功能的固定权限编号。
	PermissionPermissionUserEffectiveID = "00000000-0000-0000-0000-030200000002"

	// PermissionRoleListID 查看角色列表功能的固定权限编号。
	PermissionRoleListID = "00000000-0000-0000-0000-030300000001"
	// PermissionRoleCreateID 创建新角色功能的固定权限编号。
	PermissionRoleCreateID = "00000000-0000-0000-0000-030300000002"
	// PermissionRoleGetID 查看单个角色详情功能的固定权限编号。
	PermissionRoleGetID = "00000000-0000-0000-0000-030300000003"
	// PermissionRoleUpdateID 修改角色信息功能的固定权限编号。
	PermissionRoleUpdateID = "00000000-0000-0000-0000-030300000004"
	// PermissionRoleStatusID 启用或停用角色状态功能的固定权限编号。
	PermissionRoleStatusID = "00000000-0000-0000-0000-030300000005"

	// PermissionUserRoleListID 查看指定用户已分配角色功能的固定权限编号。
	PermissionUserRoleListID = "00000000-0000-0000-0000-030400000001"
	// PermissionUserRoleReplaceID 重新分配用户所有角色功能的固定权限编号。
	PermissionUserRoleReplaceID = "00000000-0000-0000-0000-030400000002"
	// PermissionUserRoleAddID 给用户增加角色功能的固定权限编号。
	PermissionUserRoleAddID = "00000000-0000-0000-0000-030400000003"
	// PermissionUserRoleRemoveID 移除用户某个角色功能的固定权限编号。
	PermissionUserRoleRemoveID = "00000000-0000-0000-0000-030400000004"

	// PermissionRolePermissionListID 查看指定角色包含哪些权限功能的固定权限编号。
	PermissionRolePermissionListID = "00000000-0000-0000-0000-030500000001"
	// PermissionRolePermissionReplaceID 重新设置角色包含所有权限功能的固定权限编号。
	PermissionRolePermissionReplaceID = "00000000-0000-0000-0000-030500000002"
	// PermissionRolePermissionAddID 给角色增加权限功能的固定权限编号。
	PermissionRolePermissionAddID = "00000000-0000-0000-0000-030500000003"
	// PermissionRolePermissionRemoveID 从角色中移除某项权限功能的固定权限编号。
	PermissionRolePermissionRemoveID = "00000000-0000-0000-0000-030500000004"
)
