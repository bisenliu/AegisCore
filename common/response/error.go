package response

import (
	"errors"
	"fmt"
	"net/http"
)

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

	// CodeForbidden 表示用户无权访问资源或执行操作。
	CodeForbidden Code = 30000

	// CodeConflict 表示业务冲突或资源状态不允许当前操作。
	CodeConflict Code = 40000

	// CodeNotFound 表示请求的资源不存在。
	CodeNotFound Code = 50000

	// CodeInternalError 表示服务内部错误。
	CodeInternalError Code = 90000
)

type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status}
}

func Wrap(err error, code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status, Cause: err}
}

func BadRequestError(format string, args ...any) *Error {
	return NewError(CodeBadRequest, formatMessage(format, args), http.StatusBadRequest)
}

func ValidationFailedError(format string, args ...any) *Error {
	return NewError(CodeValidationFailed, formatMessage(format, args), http.StatusBadRequest)
}

func UnauthenticatedError(format string, args ...any) *Error {
	return NewError(CodeUnauthenticated, formatMessage(format, args), http.StatusUnauthorized)
}

func TokenInvalidError(format string, args ...any) *Error {
	return NewError(CodeTokenInvalid, formatMessage(format, args), http.StatusUnauthorized)
}

func TokenExpiredError(format string, args ...any) *Error {
	return NewError(CodeTokenExpired, formatMessage(format, args), http.StatusUnauthorized)
}

func ForbiddenError(format string, args ...any) *Error {
	return NewError(CodeForbidden, formatMessage(format, args), http.StatusForbidden)
}

func ConflictError(format string, args ...any) *Error {
	return NewError(CodeConflict, formatMessage(format, args), http.StatusConflict)
}

func NotFoundError(format string, args ...any) *Error {
	return NewError(CodeNotFound, formatMessage(format, args), http.StatusNotFound)
}

func WrapInternal(err error, publicMessage string) *Error {
	return Wrap(err, CodeInternalError, publicMessage, http.StatusInternalServerError)
}

func InternalError(err error) *Error {
	return WrapInternal(err, MessageInternalError)
}

func FromError(err error) *Error {
	if err == nil {
		return InternalError(nil)
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return InternalError(err)
}

func formatMessage(format string, args []any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
