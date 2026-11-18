package errors

// Code 定义接口响应中返回的具体业务状态码（例如 0 代表成功，20001 代表登录令牌失效）。
//
// 业务状态码属于对外的接口协议规范，与 HTTP 状态码（如 200, 400, 500）不同：
// HTTP 状态码表示网络请求与传输层面的结果，由错误大类（Kind）映射得出；业务状态码则精确指示具体的业务处理结果。
//
// 错误码分段规则：
//   - 0：处理成功（仅用于 CodeOK）。
//   - 10xxx：请求格式错误、参数解析失败或表单规则校验未通过。
//   - 20xxx：用户未登录、令牌失效、密码错误或账号状态异常等身份认证问题。
//   - 30xxx：权限不足、无权访问对应数据或接口。
//   - 40xxx：业务状态冲突（例如手机号已被注册、数据版本冲突等）。
//   - 50xxx：请求的数据或资源不存在。
//   - 60xxx：访问频次过高被限流，或单次上传数据体量超出限制。
//   - 70xxx-89xxx：预留编码段，未经规范评审不得擅自使用。
//   - 90xxx：系统内部异常、底层数据库或依赖的第三方服务故障。
//
// 新增错误码注意事项：
//   - 尽量复用已有的错误大分类（Kind），并通过具体的原因标识（Reason）进行细分。
//   - 不得为了单一模块或临时需求随意开辟新的错误码段。
//   - 新增错误大类时，必须同步更新 HTTP 状态码映射规则及相关测试用例。
//   - 系统发生内部异常时，对外只展示安全友好的提示语，严禁将底层堆栈和敏感技术细节返回给前端。
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

// 60xxx：限流、配额或用量约束。
const (
	// CodeRateLimited 表示调用方超过限流或配额约束。
	CodeRateLimited Code = 60000

	// CodeRequestBodyTooLarge 表示请求体超过服务允许的字节上限。
	CodeRequestBodyTooLarge Code = 60001
)

// 90xxx：内部错误、依赖不可用和服务端临时故障。
const (
	// CodeInternalError 表示服务内部错误。
	CodeInternalError Code = 90000

	// CodeServiceUnavailable 表示服务实例或依赖暂时不可用。
	CodeServiceUnavailable Code = 90001
)
