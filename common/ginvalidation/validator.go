package ginvalidation

import (
	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/common/validation"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Bind(validator *validation.Validator, c *gin.Context, dst any, binder Binder) error {
	if err := binder(c, dst); err != nil {
		return validator.NormalizeError(dst, err)
	}
	return validator.Validate(dst)
}

func BindOrAbort(validator *validation.Validator, c *gin.Context, dst any, binder Binder) bool {
	if err := Bind(validator, c, dst, binder); err != nil {
		failure := validation.ClassifyError(err)
		fields := []zap.Field{zap.Error(err), zap.String("path", c.Request.URL.Path)}
		if len(failure.Fields) > 0 {
			fields = append(fields, zap.Any("errors", failure.Fields))
		}
		logger.Error(c.Request.Context(), "invalid request", fields...)
		if failure.IsValidation {
			response.ValidationFailedWithErrors(c, failure.Message, failure.Fields)
		} else {
			response.BadRequest(c, failure.Message)
		}
		c.Abort()
		return false
	}
	return true
}
