package messages

const (
	// InvalidUsername 是 username/nickname 为空或无效时展示给用户的消息。
	InvalidUsername = "请输入用户名"
	// InvalidUserID 是外部用户 ID 输入无效时展示给用户的消息。
	InvalidUserID = "用户ID格式不正确，请检查后重试"
	// InvalidPassword 是密码为空或无效时展示给用户的消息。
	InvalidPassword = "请输入密码"
	// InvalidCredentials 是登录凭证失败时展示给用户的消息。
	InvalidCredentials = "用户名或密码不正确，请检查后重试"
	// UserAlreadyExists 是 username 唯一性冲突时展示给用户的消息。
	UserAlreadyExists = "用户已存在，请更换用户名后重试"
	// UserNotFound 是用户资源不存在时展示给用户的消息。
	UserNotFound = "用户不存在，请检查后重试"
	// MissingSession 是认证会话缺失、过期或被撤销时展示给用户的消息。
	MissingSession = "登录状态无效或已过期，请重新登录"
)
