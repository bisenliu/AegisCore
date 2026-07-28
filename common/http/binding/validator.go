package binding

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/aegiscore/common/http/response"
	commonroute "github.com/aegiscore/common/http/route"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/validation"
)

// Bind 执行 binder，规范化绑定错误，并校验绑定后的请求值。
func Bind(validator *validation.Validator, c *gin.Context, dst any, binder Binder) error {
	if err := binder(c, dst); err != nil {
		return validator.NormalizeError(dst, err)
	}
	return validator.Validate(dst)
}

// BindOrAbort 绑定并校验请求，失败时写入错误信封并中止 Gin context。
func BindOrAbort(validator *validation.Validator, c *gin.Context, dst any, binder Binder) bool {
	if err := Bind(validator, c, dst, binder); err != nil {
		failure := validation.ClassifyError(err)
		fields := []zap.Field{zap.Error(err), zap.String("path", commonroute.TemplateOrUnmatched(c))}
		if len(failure.Fields) > 0 {
			fields = append(fields, zap.Any("errors", failure.Fields))
		}
		logger.Warn(c.Request.Context(), "invalid request", fields...)
		if failure.IsValidation {
			// validator 规则失败会暴露字段级明细；解析和绑定失败保持为通用 bad request。
			response.ValidationFailedWithErrors(c, failure.Message, failure.Fields)
		} else {
			response.BadRequest(c, failure.Message)
		}
		c.Abort()
		return false
	}
	return true
}
