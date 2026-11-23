package messages

const (
	// 输入校验类。
	InvalidUsername = "请输入用户名"
	// InvalidUserID 用户输入的编号格式不符合规范时的提示语。
	InvalidUserID = "用户编号格式不正确，请检查后重试"
	// InvalidPassword 用户未输入密码时的提示语。
	InvalidPassword = "请输入密码" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	// InvalidPermission 权限信息选择有误时的提示语。
	InvalidPermission = "权限信息填写有误，请检查后重试"
	// InvalidRole 角色信息选择有误时的提示语。
	InvalidRole = "角色信息填写有误，请检查后重试"

	// 认证与登录类。
	InvalidCredentials = "用户名或密码不正确，请检查后重试" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	// LoginTemporarilyLocked 是连续密码失败后临时拒绝登录的提示语。
	LoginTemporarilyLocked = "登录尝试过于频繁，请稍后再试"
	// LoginTemporarilyUnavailable 是登录安全检查暂时无法完成时的提示语。
	LoginTemporarilyUnavailable = "登录服务暂时不可用，请稍后重试"
	// AuthRevocationIncomplete 退出登录操作由于网络延迟尚未完全生效时的安全提示语。
	AuthRevocationIncomplete = "退出登录尚未完全生效，请稍后重试"
	// PasswordChangeRequired 用户首次登录或重置后被要求必须修改密码时的提示语。
	PasswordChangeRequired = "为保障账号安全，请先修改密码" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	// MissingSession 用户登录状态已超时或被注销时的提示语。
	MissingSession = "登录状态已失效，请重新登录"

	// 用户类。
	UserAlreadyExists = "用户已存在，请更换用户名后重试"
	// UserNotFound 系统中查询不到该用户时的提示语。
	UserNotFound = "未找到该用户，请检查后重试"

	// 权限类。
	PermissionNotFound = "未找到该权限，请检查后重试"

	// 角色类。
	RoleAlreadyExists = "角色已存在，请检查后重试"
	// RoleNotFound 系统中查询不到该角色时的提示语。
	RoleNotFound = "未找到该角色，请检查后重试"
	// SystemRoleProtected 用户企图修改系统内置核心角色时展示的拦截提示语。
	SystemRoleProtected = "系统内置角色受保护，不能修改关键内容"
	// RoleInactive 企图分配已经停用的角色时展示的拦截提示语。
	RoleInactive = "该角色已停用，不能分配给用户"

	// 用户角色绑定类。
	UserRoleAlreadyExists = "该用户已拥有此角色"
	// UserRoleNotFound 目标用户未绑定指定角色时的提示语。
	UserRoleNotFound = "该用户未绑定此角色"

	// 角色权限绑定类。
	RolePermissionAlreadyExists = "该角色已拥有此权限"
	// RolePermissionNotFound 目标角色未绑定指定权限时的提示语。
	RolePermissionNotFound = "该角色未绑定此权限"
)
