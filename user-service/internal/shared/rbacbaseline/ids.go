package rbacbaseline

// 系统保留 ID 格式为 00000000-0000-0000-0000-TTMMSSSSSSSS。
// TT 表示类型：01=系统用户，02=系统角色，03=系统权限，09=测试、fixture、诊断预留。
// MM 表示模块：00=无模块，01=user，02=permission，03=role，04=user-role，05=role-permission。
// SSSSSSSS 是同一 TT+MM 下从 00000001 开始递增的序号；已发布或已删除的 ID 不得修改或复用。
const (
	// BootstrapSuperAdminUserID semantic=user:bootstrap-super-admin。
	BootstrapSuperAdminUserID = "00000000-0000-0000-0000-010000000001"

	// SuperAdminRoleID semantic=role:super-admin。
	SuperAdminRoleID = "00000000-0000-0000-0000-020000000001"

	// PermissionUserListID semantic=permission:user:list。
	PermissionUserListID = "00000000-0000-0000-0000-030100000001"
	// PermissionUserCreateID semantic=permission:user:create。
	PermissionUserCreateID = "00000000-0000-0000-0000-030100000002"
	// PermissionUserGetID semantic=permission:user:get。
	PermissionUserGetID = "00000000-0000-0000-0000-030100000003"

	// PermissionPermissionListID semantic=permission:permission:list。
	PermissionPermissionListID = "00000000-0000-0000-0000-030200000001"
	// PermissionPermissionUserEffectiveID semantic=permission:permission:effective-by-user。
	PermissionPermissionUserEffectiveID = "00000000-0000-0000-0000-030200000002"

	// PermissionRoleListID semantic=permission:role:list。
	PermissionRoleListID = "00000000-0000-0000-0000-030300000001"
	// PermissionRoleCreateID semantic=permission:role:create。
	PermissionRoleCreateID = "00000000-0000-0000-0000-030300000002"
	// PermissionRoleGetID semantic=permission:role:get。
	PermissionRoleGetID = "00000000-0000-0000-0000-030300000003"
	// PermissionRoleUpdateID semantic=permission:role:update。
	PermissionRoleUpdateID = "00000000-0000-0000-0000-030300000004"
	// PermissionRoleStatusID semantic=permission:role:set-status。
	PermissionRoleStatusID = "00000000-0000-0000-0000-030300000005"

	// PermissionUserRoleListID semantic=permission:user-role:list。
	PermissionUserRoleListID = "00000000-0000-0000-0000-030400000001"
	// PermissionUserRoleReplaceID semantic=permission:user-role:replace。
	PermissionUserRoleReplaceID = "00000000-0000-0000-0000-030400000002"
	// PermissionUserRoleAddID semantic=permission:user-role:add。
	PermissionUserRoleAddID = "00000000-0000-0000-0000-030400000003"
	// PermissionUserRoleRemoveID semantic=permission:user-role:remove。
	PermissionUserRoleRemoveID = "00000000-0000-0000-0000-030400000004"

	// PermissionRolePermissionListID semantic=permission:role-permission:list。
	PermissionRolePermissionListID = "00000000-0000-0000-0000-030500000001"
	// PermissionRolePermissionReplaceID semantic=permission:role-permission:replace。
	PermissionRolePermissionReplaceID = "00000000-0000-0000-0000-030500000002"
	// PermissionRolePermissionAddID semantic=permission:role-permission:add。
	PermissionRolePermissionAddID = "00000000-0000-0000-0000-030500000003"
	// PermissionRolePermissionRemoveID semantic=permission:role-permission:remove。
	PermissionRolePermissionRemoveID = "00000000-0000-0000-0000-030500000004"
)
