package messages

const (
	// 输入校验类。
	InvalidUsername   = "请输入用户名"
	InvalidUserID     = "用户编号格式不正确，请检查后重试"
	InvalidPassword   = "请输入密码" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	InvalidPermission = "权限信息填写有误，请检查后重试"
	InvalidRole       = "角色信息填写有误，请检查后重试"

	// 认证与登录类。
	InvalidCredentials       = "用户名或密码不正确，请检查后重试" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	AuthRevocationIncomplete = "退出登录尚未完全生效，请稍后重试"
	PasswordChangeRequired   = "为保障账号安全，请先修改密码" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	MissingSession           = "登录状态已失效，请重新登录"

	// 用户类。
	UserAlreadyExists = "用户已存在，请更换用户名后重试"
	UserNotFound      = "未找到该用户，请检查后重试"

	// 权限类。
	PermissionAlreadyExists   = "权限已存在，请检查后重试"
	PermissionNotFound        = "未找到该权限，请检查后重试"
	SystemPermissionProtected = "系统内置权限受保护，不能修改关键内容"

	// 角色类。
	RoleAlreadyExists   = "角色已存在，请检查后重试"
	RoleNotFound        = "未找到该角色，请检查后重试"
	SystemRoleProtected = "系统内置角色受保护，不能修改关键内容"
	RoleInactive        = "该角色已停用，不能分配给用户"

	// 用户角色绑定类。
	UserRoleAlreadyExists = "该用户已拥有此角色"
	UserRoleNotFound      = "该用户未绑定此角色"

	// 角色权限绑定类。
	RolePermissionAlreadyExists = "该角色已拥有此权限"
	RolePermissionNotFound      = "该角色未绑定此权限"
)
