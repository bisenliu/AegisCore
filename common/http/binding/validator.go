package binding

import (
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	contracterrors "github.com/aegiscore/common/contract/errors"
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
			// validator 规则失败会暴露字段级明细；其他错误保持普通失败信封。
			response.ValidationFailedWithErrors(c, failure.Message, failure.Fields)
		} else if appErr := applicationError(err); appErr != nil {
			response.WriteError(c, appErr)
		} else {
			response.BadRequest(c, failure.Message)
		}
		c.Abort()
		return false
	}
	return true
}

func applicationError(err error) *contracterrors.Error {
	var appErr *contracterrors.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}
