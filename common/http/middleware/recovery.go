package middleware

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/http/response"
	"github.com/aegiscore/common/runtime/logger"
)

// Recovery 兜底捕获未处理的运行时异常（panic），自动转换为友好的系统内部错误响应，并将异常详情与调用堆栈记录到日志中，防止程序意外崩溃。
func Recovery(log *zap.Logger) gin.HandlerFunc {
	recoveryLog := logger.NamedComponent(log, "http", "recovery")
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				recordPanicOnSpan(c.Request.Context())
				logger.WithContext(c.Request.Context(), recoveryLog).Error("panic recovered",
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				response.Fail(c, contracterrors.InternalError(nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}
