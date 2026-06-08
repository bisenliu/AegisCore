package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BadRequest 写入表示请求格式错误或无法解析的 400 失败信封。
func BadRequest(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeBadRequest, formatMessage(format, args), http.StatusBadRequest))
}

// ValidationFailed 写入表示请求字段语义校验失败的 400 失败信封。
func ValidationFailed(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeValidationFailed, formatMessage(format, args), http.StatusBadRequest))
}

// ValidationFailedWithErrors 写入包含结构化字段明细的 400 校验失败信封。
func ValidationFailedWithErrors(c *gin.Context, message string, errors any) {
	c.JSON(http.StatusBadRequest, Envelope{Success: false, Code: CodeValidationFailed, Message: message, Errors: errors})
}

// Unauthenticated 写入表示缺少认证状态的 401 失败信封。
func Unauthenticated(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeUnauthenticated, formatMessage(format, args), http.StatusUnauthorized))
}

// TokenInvalid 写入表示 token 格式错误或无法校验的 401 失败信封。
func TokenInvalid(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeTokenInvalid, formatMessage(format, args), http.StatusUnauthorized))
}

// TokenExpired 写入表示 token 已过期的 401 失败信封。
func TokenExpired(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeTokenExpired, formatMessage(format, args), http.StatusUnauthorized))
}

// Forbidden 写入表示认证调用方无权限的 403 失败信封。
func Forbidden(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeForbidden, formatMessage(format, args), http.StatusForbidden))
}

// Conflict 写入表示领域冲突或资源状态不允许操作的 409 失败信封。
func Conflict(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeConflict, formatMessage(format, args), http.StatusConflict))
}

// NotFound 写入表示资源不存在的 404 失败信封。
func NotFound(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeNotFound, formatMessage(format, args), http.StatusNotFound))
}
