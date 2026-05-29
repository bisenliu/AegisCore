package middleware

import (
	"runtime/debug"

	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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
