package rbacbaseline

const (
	// SystemIDNamespace 是系统内置 RBAC ID 的 UUID v5 namespace，项目初始化后固化；已有项目重命名时不得自动修改。
	SystemIDNamespace = "8f6d2f52-7c0e-4a88-b8a8-6dcb2d7c9c19"

	// SuperAdminRoleID 由 UUIDv5(SystemIDNamespace, "role:super-admin") 生成后固化。
	SuperAdminRoleID = "425a7018-7d64-5715-a78c-270c4c95a545"

	// BootstrapSuperAdminUserID 由 UUIDv5(SystemIDNamespace, "user:bootstrap-super-admin") 生成后固化。
	BootstrapSuperAdminUserID = "ca4a61e8-0480-5747-9279-43ad08ff2779"

	// PermissionUserListID 由 UUIDv5(SystemIDNamespace, "permission:user:list") 生成后固化。
	PermissionUserListID = "e6c89398-8525-585f-9cd1-d623c01e7873"
	// PermissionUserCreateID 由 UUIDv5(SystemIDNamespace, "permission:user:create") 生成后固化。
	PermissionUserCreateID = "c8695787-36e6-53de-bc85-1162e3c813f5"
	// PermissionUserGetID 由 UUIDv5(SystemIDNamespace, "permission:user:get") 生成后固化。
	PermissionUserGetID = "8e2c9b12-0f7e-5042-8788-ee01541206c0"

	// PermissionPermissionListID 由 UUIDv5(SystemIDNamespace, "permission:permission:list") 生成后固化。
	PermissionPermissionListID = "3cedd373-41de-5afe-9e32-4c7efc08ce41"
	// PermissionPermissionUserEffectiveID 由 UUIDv5(SystemIDNamespace, "permission:permission:effective-by-user") 生成后固化。
	PermissionPermissionUserEffectiveID = "58a94cab-4fb4-51ee-b77a-8d59374e6a75"

	// PermissionRoleListID 由 UUIDv5(SystemIDNamespace, "permission:role:list") 生成后固化。
	PermissionRoleListID = "d3c34b46-a184-54d4-b135-4ce906f7110f"
	// PermissionRoleCreateID 由 UUIDv5(SystemIDNamespace, "permission:role:create") 生成后固化。
	PermissionRoleCreateID = "3e33aa30-f6bb-5528-8e29-1ad55df5827d"
	// PermissionRoleGetID 由 UUIDv5(SystemIDNamespace, "permission:role:get") 生成后固化。
	PermissionRoleGetID = "5bc74065-a105-5d81-aabb-3d7a7ce75c56"
	// PermissionRoleUpdateID 由 UUIDv5(SystemIDNamespace, "permission:role:update") 生成后固化。
	PermissionRoleUpdateID = "18951f76-1a1d-5789-b4ca-25953768c543"
	// PermissionRoleStatusID 由 UUIDv5(SystemIDNamespace, "permission:role:set-status") 生成后固化。
	PermissionRoleStatusID = "e32bb7cb-334a-5b28-8f28-59d9764d1d48"

	// PermissionUserRoleListID 由 UUIDv5(SystemIDNamespace, "permission:user-role:list") 生成后固化。
	PermissionUserRoleListID = "6d7fc228-21bd-5176-aa65-746dc4575b49"
	// PermissionUserRoleReplaceID 由 UUIDv5(SystemIDNamespace, "permission:user-role:replace") 生成后固化。
	PermissionUserRoleReplaceID = "425f490d-6ce5-558b-b782-7bd67d6a4f04"
	// PermissionUserRoleAddID 由 UUIDv5(SystemIDNamespace, "permission:user-role:add") 生成后固化。
	PermissionUserRoleAddID = "cef73730-9c12-558f-b948-ab2d660c7260"
	// PermissionUserRoleRemoveID 由 UUIDv5(SystemIDNamespace, "permission:user-role:remove") 生成后固化。
	PermissionUserRoleRemoveID = "b78591a1-fe96-58f3-88ab-00eeea432622"

	// PermissionRolePermissionListID 由 UUIDv5(SystemIDNamespace, "permission:role-permission:list") 生成后固化。
	PermissionRolePermissionListID = "386ed52f-c3a2-503c-b73c-edef953a570f"
	// PermissionRolePermissionReplaceID 由 UUIDv5(SystemIDNamespace, "permission:role-permission:replace") 生成后固化。
	PermissionRolePermissionReplaceID = "86c12547-5f7e-56f9-be8e-2157b88d31d4"
	// PermissionRolePermissionAddID 由 UUIDv5(SystemIDNamespace, "permission:role-permission:add") 生成后固化。
	PermissionRolePermissionAddID = "930d1b93-1148-53ce-90a5-8be78d140d9f"
	// PermissionRolePermissionRemoveID 由 UUIDv5(SystemIDNamespace, "permission:role-permission:remove") 生成后固化。
	PermissionRolePermissionRemoveID = "d611f69c-9b1d-508d-8feb-be0983d802f6"
)
