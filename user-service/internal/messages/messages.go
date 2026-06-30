package messages

const (
	// InvalidUsername 是 username/nickname 为空或无效时展示给用户的消息。
	InvalidUsername = "请输入用户名"
	// InvalidUserID 是外部用户 ID 输入无效时展示给用户的消息。
	InvalidUserID = "用户ID格式不正确，请检查后重试"
	// InvalidPassword 是密码为空或无效时展示给用户的消息。
	InvalidPassword = "请输入密码" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	// InvalidCredentials 是登录凭证失败时展示给用户的消息。
	InvalidCredentials = "用户名或密码不正确，请检查后重试" // #nosec G101 -- 用户提示文案，不包含真实凭据。
	// AuthServiceBusy 是认证服务资源繁忙时展示给用户的消息。
	AuthServiceBusy = "认证服务繁忙，请稍后重试"
	// UserAlreadyExists 是 username 唯一性冲突时展示给用户的消息。
	UserAlreadyExists = "用户已存在，请更换用户名后重试"
	// UserNotFound 是用户资源不存在时展示给用户的消息。
	UserNotFound = "用户不存在，请检查后重试"
	// MissingSession 是认证会话缺失、过期或被撤销时展示给用户的消息。
	MissingSession = "登录状态无效或已过期，请重新登录"
	// InvalidPermission 是权限目录输入无效时展示给用户的消息。
	InvalidPermission = "权限参数不正确，请检查后重试"
	// PermissionAlreadyExists 是权限唯一性冲突时展示给用户的消息。
	PermissionAlreadyExists = "权限已存在，请检查 HTTP 方法和路径模板"
	// PermissionNotFound 是权限资源不存在时展示给用户的消息。
	PermissionNotFound = "权限不存在，请检查后重试"
	// SystemPermissionProtected 是系统权限受保护字段被修改时展示给用户的消息。
	SystemPermissionProtected = "系统权限受保护，不能修改关键字段"
	// InvalidRole 是角色输入无效时展示给用户的消息。
	InvalidRole = "角色参数不正确，请检查后重试"
	// RoleAlreadyExists 是角色唯一性冲突时展示给用户的消息。
	RoleAlreadyExists = "角色已存在，请检查后重试"
	// RoleNotFound 是角色资源不存在时展示给用户的消息。
	RoleNotFound = "角色不存在，请检查后重试"
	// SystemRoleProtected 是系统角色受保护字段被修改时展示给用户的消息。
	SystemRoleProtected = "系统角色受保护，不能修改关键字段"
	// UserRoleAlreadyExists 是用户角色绑定重复时展示给用户的消息。
	UserRoleAlreadyExists = "用户角色绑定已存在"
	// UserRoleNotFound 是用户角色绑定不存在时展示给用户的消息。
	UserRoleNotFound = "用户角色绑定不存在"
	// RolePermissionAlreadyExists 是角色权限绑定重复时展示给用户的消息。
	RolePermissionAlreadyExists = "角色权限绑定已存在"
	// RolePermissionNotFound 是角色权限绑定不存在时展示给用户的消息。
	RolePermissionNotFound = "角色权限绑定不存在"
)
