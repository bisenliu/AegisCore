package errors

import (
	"fmt"
	"net/http"
)

const (
	// MessageInternalError 是对外暴露的非敏感服务器错误消息。
	MessageInternalError = "internal server error"
)

// NewError 创建不包含内部原因的应用错误。
func NewError(code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status}
}

// Wrap 创建应用错误，并将 err 保留为内部原因。
func Wrap(err error, code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status, Cause: err}
}

// BadRequestError 创建表示请求格式错误或无法解析的 400 错误。
func BadRequestError(format string, args ...any) *Error {
	return NewError(CodeBadRequest, formatMessage(format, args), http.StatusBadRequest)
}

// ValidationFailedError 创建表示请求字段语义校验失败的 400 错误。
func ValidationFailedError(format string, args ...any) *Error {
	return NewError(CodeValidationFailed, formatMessage(format, args), http.StatusBadRequest)
}

// UnauthenticatedError 创建表示缺少认证状态的 401 错误。
func UnauthenticatedError(format string, args ...any) *Error {
	return NewError(CodeUnauthenticated, formatMessage(format, args), http.StatusUnauthorized)
}

// TokenInvalidError 创建表示 token 格式错误、无效或无法校验的 401 错误。
func TokenInvalidError(format string, args ...any) *Error {
	return NewError(CodeTokenInvalid, formatMessage(format, args), http.StatusUnauthorized)
}

// TokenExpiredError 创建表示 token 已过期而被拒绝的 401 错误。
func TokenExpiredError(format string, args ...any) *Error {
	return NewError(CodeTokenExpired, formatMessage(format, args), http.StatusUnauthorized)
}

// ForbiddenError 创建表示认证调用方无权访问资源的 403 错误。
func ForbiddenError(format string, args ...any) *Error {
	return NewError(CodeForbidden, formatMessage(format, args), http.StatusForbidden)
}

// ConflictError 创建表示领域冲突或资源状态不允许操作的 409 错误。
func ConflictError(format string, args ...any) *Error {
	return NewError(CodeConflict, formatMessage(format, args), http.StatusConflict)
}

// NotFoundError 创建表示资源不存在或不可见的 404 错误。
func NotFoundError(format string, args ...any) *Error {
	return NewError(CodeNotFound, formatMessage(format, args), http.StatusNotFound)
}

// WrapInternal 创建 500 错误，并在响应中使用 publicMessage 隐藏 err 细节。
func WrapInternal(err error, publicMessage string) *Error {
	return Wrap(err, CodeInternalError, publicMessage, http.StatusInternalServerError)
}

// InternalError 创建使用包默认非敏感公开消息的 500 错误。
func InternalError(err error) *Error {
	return WrapInternal(err, MessageInternalError)
}

func formatMessage(format string, args []any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
