package errors

import (
	"fmt"
)

const (
	// MessageInternalError 是对外暴露的非敏感服务器错误消息。
	MessageInternalError = "internal server error"
	// MessageRequestBodyTooLarge 是请求体超过容量边界时的公开消息。
	MessageRequestBodyTooLarge = "请求体过大"
)

// New 创建不包含内部原因的语义应用错误。
func New(kind Kind, reason Reason, code Code, message string) *Error {
	return &Error{Kind: kind, Reason: reason, Code: code, Message: message}
}

// Wrap 创建语义应用错误，并将 err 保留为内部原因。
func Wrap(err error, kind Kind, reason Reason, code Code, message string) *Error {
	return &Error{Kind: kind, Reason: reason, Code: code, Message: message, Cause: err}
}

// BadRequestError 创建表示请求格式错误或无法解析的应用错误。
func BadRequestError(format string, args ...any) *Error {
	return New(KindBadRequest, ReasonBadRequest, CodeBadRequest, formatMessage(format, args))
}

// ValidationFailedError 创建表示请求字段语义校验失败的应用错误。
func ValidationFailedError(format string, args ...any) *Error {
	return New(KindValidation, ReasonValidationFailed, CodeValidationFailed, formatMessage(format, args))
}

// UnauthenticatedError 创建表示缺少认证状态的应用错误。
func UnauthenticatedError(format string, args ...any) *Error {
	return New(KindUnauthenticated, ReasonUnauthenticated, CodeUnauthenticated, formatMessage(format, args))
}

// TokenInvalidError 创建表示 token 格式错误、无效或无法校验的应用错误。
func TokenInvalidError(format string, args ...any) *Error {
	return New(KindUnauthenticated, ReasonTokenInvalid, CodeTokenInvalid, formatMessage(format, args))
}

// TokenExpiredError 创建表示 token 已过期而被拒绝的应用错误。
func TokenExpiredError(format string, args ...any) *Error {
	return New(KindUnauthenticated, ReasonTokenExpired, CodeTokenExpired, formatMessage(format, args))
}

// ForbiddenError 创建表示认证调用方无权访问资源的应用错误。
func ForbiddenError(format string, args ...any) *Error {
	return New(KindForbidden, ReasonForbidden, CodeForbidden, formatMessage(format, args))
}

// ConflictError 创建表示领域冲突或资源状态不允许操作的应用错误。
func ConflictError(format string, args ...any) *Error {
	return New(KindConflict, ReasonConflict, CodeConflict, formatMessage(format, args))
}

// NotFoundError 创建表示资源不存在或不可见的应用错误。
func NotFoundError(format string, args ...any) *Error {
	return New(KindNotFound, ReasonNotFound, CodeNotFound, formatMessage(format, args))
}

// RateLimitedError 创建表示请求超过限流或配额约束的应用错误。
func RateLimitedError(format string, args ...any) *Error {
	return New(KindRateLimited, ReasonRateLimited, CodeRateLimited, formatMessage(format, args))
}

// RequestBodyTooLargeError 创建表示请求体超过服务字节上限的应用错误。
func RequestBodyTooLargeError() *Error {
	return New(KindPayloadTooLarge, ReasonRequestBodyTooLarge, CodeRequestBodyTooLarge, MessageRequestBodyTooLarge)
}

// WrapRequestBodyTooLarge 创建请求体超限应用错误，并保留底层读取错误。
func WrapRequestBodyTooLarge(err error) *Error {
	return Wrap(err, KindPayloadTooLarge, ReasonRequestBodyTooLarge, CodeRequestBodyTooLarge, MessageRequestBodyTooLarge)
}

// ServiceUnavailableError 创建表示服务实例或依赖暂时不可用的应用错误。
func ServiceUnavailableError(format string, args ...any) *Error {
	return New(KindServiceUnavailable, ReasonServiceUnavailable, CodeServiceUnavailable, formatMessage(format, args))
}

// WrapServiceUnavailable 创建服务不可用应用错误，并将 err 保留为内部原因。
func WrapServiceUnavailable(err error, publicMessage string) *Error {
	return Wrap(err, KindServiceUnavailable, ReasonServiceUnavailable, CodeServiceUnavailable, publicMessage)
}

// WrapInternal 创建内部应用错误，并在响应中使用 publicMessage 隐藏 err 细节。
func WrapInternal(err error, publicMessage string) *Error {
	return Wrap(err, KindInternal, ReasonInternalError, CodeInternalError, publicMessage)
}

// InternalError 创建使用包默认非敏感公开消息的内部应用错误。
func InternalError(err error) *Error {
	return WrapInternal(err, MessageInternalError)
}

func formatMessage(format string, args []any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
