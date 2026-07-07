package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/common/contract/errors"
)

// Fail 将 err 转换为应用错误，并写入对应失败响应信封。
func Fail(c *gin.Context, err error) {
	appErr := contracterrors.FromError(err)
	// 字段级校验明细由 ValidationFailedWithErrors 输出，通用失败保持信封简洁。
	WriteError(c, appErr)
}

// WriteError 写入已归一化的应用错误。
func WriteError(c *gin.Context, err *contracterrors.Error) {
	err = normalizeAppError(err)
	annotateAppErrorOnSpan(c.Request.Context(), err)
	JSON(c, statusCode(err), errorEnvelope(err))
}

func normalizeAppError(err *contracterrors.Error) *contracterrors.Error {
	if err == nil {
		return contracterrors.InternalError(nil)
	}
	if statusCode(err) == http.StatusInternalServerError && err.Kind != contracterrors.KindInternal {
		return contracterrors.InternalError(err)
	}
	return err
}

// BadRequest 写入表示请求格式错误或无法解析的 400 失败信封。
func BadRequest(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.BadRequestError(format, args...))
}

// ValidationFailed 写入表示请求字段语义校验失败的 400 失败信封。
func ValidationFailed(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.ValidationFailedError(format, args...))
}

// ValidationFailedWithErrors 写入包含结构化字段明细的 400 校验失败信封。
func ValidationFailedWithErrors(c *gin.Context, message string, errors any) {
	appErr := contracterrors.ValidationFailedError(message)
	annotateAppErrorOnSpan(c.Request.Context(), appErr)
	JSON(c, statusCode(appErr), validationEnvelope(message, errors))
}

// Unauthenticated 写入表示缺少认证状态的 401 失败信封。
func Unauthenticated(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.UnauthenticatedError(format, args...))
}

// TokenInvalid 写入表示 token 格式错误或无法校验的 401 失败信封。
func TokenInvalid(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.TokenInvalidError(format, args...))
}

// TokenExpired 写入表示 token 已过期的 401 失败信封。
func TokenExpired(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.TokenExpiredError(format, args...))
}

// Forbidden 写入表示认证调用方无权限的 403 失败信封。
func Forbidden(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.ForbiddenError(format, args...))
}

// Conflict 写入表示领域冲突或资源状态不允许操作的 409 失败信封。
func Conflict(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.ConflictError(format, args...))
}

// NotFound 写入表示资源不存在的 404 失败信封。
func NotFound(c *gin.Context, format string, args ...any) {
	WriteError(c, contracterrors.NotFoundError(format, args...))
}
