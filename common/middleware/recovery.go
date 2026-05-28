package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
)

func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(c.Request.Context(), "panic recovered",
					slog.Any("panic", r),
					slog.String("request_id", requestID(c)),
					slog.String("stack", string(debug.Stack())),
				)
				response.Fail(c, response.InternalError(nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}
