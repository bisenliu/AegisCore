package middleware

import (
	"runtime/debug"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 将 panic 转换为内部错误响应信封，并记录 panic 与堆栈明细。
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithContext(log, c.Request.Context()).Error("panic recovered",
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				response.Fail(c, response.InternalError(nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}
