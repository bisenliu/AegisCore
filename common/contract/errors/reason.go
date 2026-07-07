package errors

// Reason 表达稳定、可公开的具体错误原因。
type Reason string

const (
	// ReasonBadRequest 表示通用请求格式错误。
	ReasonBadRequest Reason = "bad_request"
	// ReasonRequestBindingFailed 表示请求字段绑定或类型转换失败。
	ReasonRequestBindingFailed Reason = "request_binding_failed"
	// ReasonEmptyRequestBody 表示请求需要 body 但内容为空。
	ReasonEmptyRequestBody Reason = "empty_request_body"
	// ReasonTrailingJSONBody 表示 JSON body 包含多个 JSON 值。
	ReasonTrailingJSONBody Reason = "trailing_json_body"
	// ReasonValidationFailed 表示请求字段语义校验失败。
	ReasonValidationFailed Reason = "validation_failed"
	// ReasonUnauthenticated 表示缺少认证状态。
	ReasonUnauthenticated Reason = "unauthenticated"
	// ReasonTokenInvalid 表示 token 格式错误、非法或无法校验。
	ReasonTokenInvalid Reason = "token_invalid"
	// ReasonTokenExpired 表示 token 已过期。
	ReasonTokenExpired Reason = "token_expired"
	// ReasonTokenRevoked 表示 token 已撤销。
	ReasonTokenRevoked Reason = "token_revoked"
	// ReasonMFARequired 表示需要多因素认证。
	ReasonMFARequired Reason = "mfa_required"
	// ReasonUserAccountLocked 表示用户账号被冻结或封禁。
	ReasonUserAccountLocked Reason = "user_account_locked"
	// ReasonForbidden 表示权限不足。
	ReasonForbidden Reason = "forbidden"
	// ReasonConflict 表示领域冲突。
	ReasonConflict Reason = "conflict"
	// ReasonNotFound 表示资源不存在或不可见。
	ReasonNotFound Reason = "not_found"
	// ReasonInternalError 表示服务内部错误。
	ReasonInternalError Reason = "internal_error"
	// ReasonServiceUnavailable 表示服务实例或依赖暂时不可用。
	ReasonServiceUnavailable Reason = "service_unavailable"
)
