package response

import (
	"errors"
	"fmt"
	"net/http"
)

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

	// CodeForbidden 表示用户无权访问资源或执行操作。
	CodeForbidden Code = 30000

	// CodeConflict 表示业务冲突或资源状态不允许当前操作。
	CodeConflict Code = 40000

	// CodeNotFound 表示请求的资源不存在。
	CodeNotFound Code = 50000

	// CodeInternalError 表示服务内部错误。
	CodeInternalError Code = 90000
)

// Error 描述可渲染为响应信封的应用错误。
// Cause 保留内部错误供 errors.Is/As 使用，但不会暴露给客户端。
type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	Cause      error
}

// Error 返回对外展示的错误消息，并允许 nil receiver 以兼容防御性日志路径。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Unwrap 返回内部原因错误，使调用方可以使用 errors.Is 和 errors.As。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

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

// FromError 将任意错误转换为应用错误，nil 和未知错误默认映射为内部错误。
func FromError(err error) *Error {
	if err == nil {
		// nil 错误无法生成成功的失败信封，因此按失败关闭策略映射为内部错误。
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
