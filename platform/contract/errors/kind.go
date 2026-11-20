package errors

// Kind 定义错误所属的大分类（例如参数错误、权限不足、系统内部异常等），接口层会根据此类分类自动决定返回给前端的 HTTP 状态码（如 400、401、403、500 等）。
type Kind string

const (
	// KindBadRequest 表示请求格式错误或无法解析。
	KindBadRequest Kind = "bad_request"
	// KindValidation 表示提交的表单或请求参数格式及规则校验未通过。
	KindValidation Kind = "validation_failed"
	// KindUnauthenticated 表示调用方缺少有效认证状态。
	KindUnauthenticated Kind = "unauthenticated"
	// KindForbidden 表示认证调用方无权执行操作。
	KindForbidden Kind = "forbidden"
	// KindConflict 表示操作与当前数据状态发生冲突（如名称重复或正在被修改）。
	KindConflict Kind = "conflict"
	// KindNotFound 表示资源不存在或不可见。
	KindNotFound Kind = "not_found"
	// KindRateLimited 表示请求过于频繁，超过了系统允许的访问频率限制。
	KindRateLimited Kind = "rate_limited"
	// KindPayloadTooLarge 表示上传或提交的数据体量过大，超过了系统限制。
	KindPayloadTooLarge Kind = "payload_too_large"
	// KindInternal 表示服务内部错误。
	KindInternal Kind = "internal"
	// KindServiceUnavailable 表示服务实例或依赖暂时不可用。
	KindServiceUnavailable Kind = "service_unavailable"
)
