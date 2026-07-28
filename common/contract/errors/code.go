package errors

// Code 是所有响应信封携带的稳定应用层响应码。
//
// Code 是公开 API 契约，不等同于 HTTP status；HTTP status 只能由 Kind 推导。
//
// 错误码段分配：
//   - 0：成功，只用于 CodeOK。
//   - 10xxx：请求解析、绑定和字段校验错误。
//   - 20xxx：认证、凭证、Token、session 和账号登录态错误。
//   - 30xxx：授权、访问控制和策略拒绝错误。
//   - 40xxx：业务冲突、资源状态不允许和幂等冲突错误。
//   - 50xxx：资源不存在或不可见错误。
//   - 60xxx：预留给限流、配额或用量约束；启用前必须先定义 Kind、HTTP 映射和测试。
//   - 70xxx-89xxx：预留，未经规格变更不得使用。
//   - 90xxx：内部错误、依赖不可用和服务端临时故障。
//
// 新增错误码规则：
//   - 必须优先复用现有低基数 Kind，并使用稳定 Reason 表达可细分原因。
//   - 不得按 feature、目录、临时实现任务或调用方便利随意开辟错误码段。
//   - 新增 Kind 必须同步 common/http/response.statusCode 和响应测试。
//   - 内部错误对外必须使用非敏感公开消息，不得把 Cause 暴露到响应 envelope。
type Code int

// 成功响应。
const (
	// CodeOK 表示请求成功。
	CodeOK Code = 0
)

// 10xxx：请求解析、绑定和字段校验错误。
const (
	// CodeBadRequest 表示通用请求错误，例如请求体格式错误、参数无法解析。
	CodeBadRequest Code = 10000

	// CodeValidationFailed 表示请求参数校验失败，例如必填、长度、范围、格式或枚举规则不通过。
	CodeValidationFailed Code = 10001
)

// 20xxx：认证、凭证、Token、session 和账号登录态错误。
const (
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
)

// 30xxx：授权、访问控制和策略拒绝错误。
const (
	// CodeForbidden 表示用户无权访问资源或执行操作。
	CodeForbidden Code = 30000
)

// 40xxx：业务冲突、资源状态不允许和幂等冲突错误。
const (
	// CodeConflict 表示业务冲突或资源状态不允许当前操作。
	CodeConflict Code = 40000
)

// 50xxx：资源不存在或不可见错误。
const (
	// CodeNotFound 表示请求的资源不存在。
	CodeNotFound Code = 50000
)

// 90xxx：内部错误、依赖不可用和服务端临时故障。
const (
	// CodeInternalError 表示服务内部错误。
	CodeInternalError Code = 90000

	// CodeServiceUnavailable 表示服务实例或依赖暂时不可用。
	CodeServiceUnavailable Code = 90001
)
