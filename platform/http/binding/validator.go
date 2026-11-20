package binding

import (
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	contracterrors "github.com/aegiscore/platform/contract/errors"
	"github.com/aegiscore/platform/http/response"
	platformroute "github.com/aegiscore/platform/http/route"
	"github.com/aegiscore/platform/runtime/logger"
	"github.com/aegiscore/platform/validation"
)

// Bind 执行 binder，规范化绑定错误，并校验绑定后的请求值。
func Bind(validator *validation.Validator, c *gin.Context, dst any, binder Binder) error {
	if err := binder(c, dst); err != nil {
		return validator.NormalizeError(dst, err)
	}
	return validator.Validate(dst)
}

// BindOrAbort 解析并校验请求参数；若参数格式错误或校验不通过，则直接向客户端输出错误响应并终止后续处理。
func BindOrAbort(validator *validation.Validator, c *gin.Context, dst any, binder Binder) bool {
	if err := Bind(validator, c, dst, binder); err != nil {
		failure := validation.ClassifyError(err)
		fields := []zap.Field{zap.Error(err), zap.String("path", platformroute.TemplateOrUnmatched(c))}
		if len(failure.Fields) > 0 {
			fields = append(fields, zap.Any("errors", failure.Fields))
		}
		// 请求解析失败或参数校验未通过属于客户端输入问题（HTTP 400/422），系统已安全拦截，并非服务端内部故障。
		// 此处使用 Warn 级别记录具体的校验失败字段与路由，便于前后端联调排查和异常流量感知；严禁记录为 Error 级别，避免将用户输入手误或外部扫描探测误当成系统故障触发线上报警风暴。
		logger.Warn(c.Request.Context(), "invalid request", fields...)
		if failure.IsValidation {
			// 若是表单或字段规则未通过，则返回详细的字段错误清单（如提示具体哪个字段超长或为空）；若是请求解析失败等通用问题，则返回标准错误提示。
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

// applicationError 将数据检查或参数提取时的错误，统一整理成客户端可直接识别的标准业务错误格式。
func applicationError(err error) *contracterrors.Error {
	var appErr *contracterrors.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}
