package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/platform/contract/errors"
)

// Fail 将业务错误统一转换为标准错误格式，并输出标准的 JSON 失败响应。
func Fail(c *gin.Context, err error) {
	appErr := contracterrors.FromError(err)
	// 具体的字段校验错误明细由专门的 ValidationFailedWithErrors 接口输出，通用的业务失败只返回整体错误提示，保持接口响应结构简洁明了。
	WriteError(c, appErr)
}

// WriteError 将规范化后的业务错误渲染为 HTTP 错误响应并返回给客户端。
func WriteError(c *gin.Context, err *contracterrors.Error) {
	err = normalizeAppError(err)
	annotateAppErrorOnSpan(c.Request.Context(), err)
	JSON(c, statusCode(err), errorEnvelope(err))
}

// normalizeAppError 确保返回给用户的错误信息安全得体；如果内部错误没有预设公开提示语，则补充通用提示，严禁将底层代码报错直接抛给外部。
func normalizeAppError(err *contracterrors.Error) *contracterrors.Error {
	if err == nil {
		return contracterrors.InternalError(nil)
	}
	if statusCode(err) == http.StatusInternalServerError && err.Kind != contracterrors.KindInternal {
		return contracterrors.InternalError(err)
	}
	return err
}

// BadRequest 向客户端返回 HTTP 400 错误响应（表示请求参数格式不正确或解析失败）。
func BadRequest(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.BadRequestError(format, args...))
}

// ValidationFailed 向客户端返回 HTTP 400 校验失败响应（表示提交的表单字段未通过规则检查）。
func ValidationFailed(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.ValidationFailedError(format, args...))
}

// ValidationFailedWithErrors 向客户端返回 HTTP 400 校验失败响应，并携带各个具体字段的错误详情（例如哪些字段为空或超出范围）。
func ValidationFailedWithErrors(c *gin.Context, message string, errors any) {
	appErr := contracterrors.ValidationFailedError(message)
	annotateAppErrorOnSpan(c.Request.Context(), appErr)
	JSON(c, statusCode(appErr), validationEnvelope(message, errors))
}

// Unauthenticated 向客户端返回 HTTP 401 未登录响应。
func Unauthenticated(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.UnauthenticatedError(format, args...))
}

// TokenInvalid 向客户端返回 HTTP 401 令牌无效响应（表示登录凭证已被篡改、失效或非法）。
func TokenInvalid(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.TokenInvalidError(format, args...))
}

// TokenExpired 向客户端返回 HTTP 401 令牌过期响应。
func TokenExpired(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.TokenExpiredError(format, args...))
}

// Forbidden 向客户端返回 HTTP 403 权限不足响应。
func Forbidden(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.ForbiddenError(format, args...))
}

// Conflict 向客户端返回 HTTP 409 状态冲突响应（例如数据已被占用或状态不允许执行）。
func Conflict(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.ConflictError(format, args...))
}

// NotFound 向客户端返回 HTTP 404 资源未找到响应。
func NotFound(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.NotFoundError(format, args...))
}

// RateLimited 向客户端返回 HTTP 429 访问受限响应（表示请求过于频繁被系统限流）。
func RateLimited(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.RateLimitedError(format, args...))
}

// PayloadTooLarge 向客户端返回 HTTP 413 请求体超限响应（表示提交的数据过大）。
func PayloadTooLarge(c *gin.Context) {
	WriteError(c, contracterrors.RequestBodyTooLargeError())
}
