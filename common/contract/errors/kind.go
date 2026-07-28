package errors

// Kind 表达低基数应用错误类别，由 HTTP 层统一推导状态码。
type Kind string

const (
	// KindBadRequest 表示请求格式错误或无法解析。
	KindBadRequest Kind = "bad_request"
	// KindValidation 表示请求字段语义校验失败。
	KindValidation Kind = "validation_failed"
	// KindUnauthenticated 表示调用方缺少有效认证状态。
	KindUnauthenticated Kind = "unauthenticated"
	// KindForbidden 表示认证调用方无权执行操作。
	KindForbidden Kind = "forbidden"
	// KindConflict 表示领域冲突或资源状态不允许操作。
	KindConflict Kind = "conflict"
	// KindNotFound 表示资源不存在或不可见。
	KindNotFound Kind = "not_found"
	// KindRateLimited 表示调用方超过限流或配额约束。
	KindRateLimited Kind = "rate_limited"
	// KindInternal 表示服务内部错误。
	KindInternal Kind = "internal"
	// KindServiceUnavailable 表示服务实例或依赖暂时不可用。
	KindServiceUnavailable Kind = "service_unavailable"
)
