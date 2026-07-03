package errors

// Code 是所有响应信封携带的稳定应用层响应码。
type Code int

const (
	// CodeOK 表示请求成功。
	CodeOK Code = 0

	// CodeBadRequest 表示通用请求错误，例如请求体格式错误、参数无法解析。
	CodeBadRequest Code = 10000

	// CodeValidationFailed 表示请求参数校验失败，例如必填、长度、范围、格式或枚举规则不通过。
	CodeValidationFailed Code = 10001

	// CodeUnauthenticated 表示用户未认证。
	CodeUnauthenticated Code = 20000

	// CodeTokenInvalid 表示 Token 格式错误、非法或签名解析失败。
	CodeTokenInvalid Code = 20001

	// CodeTokenExpired 表示 Token 已过期。
	CodeTokenExpired Code = 20002

	// CodeTokenRevoked 表示 Token 已失效/被拉黑（如用户在别处修改了密码、或主动登出）。
	// 前端捕获后应清空本地缓存，直接重定向至登录页。
	CodeTokenRevoked Code = 20003

	// CodeMFARequired 表示密码验证通过，但需要进行多因素认证（MFA，如短信验证码、Authenticator）。
	CodeMFARequired Code = 20004

	// CodeUserAccountLocked 表示用户账号已被冻结或封禁。
	CodeUserAccountLocked Code = 20005

	// CodePasswordChangeRequired 表示用户凭据有效，但必须先完成密码修改。
	CodePasswordChangeRequired Code = 20006

	// CodeForbidden 表示用户无权访问资源或执行操作。
	CodeForbidden Code = 30000

	// CodeConflict 表示业务冲突或资源状态不允许当前操作。
	CodeConflict Code = 40000

	// CodeNotFound 表示请求的资源不存在。
	CodeNotFound Code = 50000

	// CodeInternalError 表示服务内部错误。
	CodeInternalError Code = 90000

	// CodeServiceUnavailable 表示服务实例或依赖暂时不可用。
	CodeServiceUnavailable Code = 90001
)
