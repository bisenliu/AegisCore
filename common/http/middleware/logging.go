package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
)

const anonymousUserID = "anonymous"

// RequestLogger 使用携带 trace 的 context 记录每个完成的 HTTP 请求，并按状态码选择日志级别。
func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		reqLog := logger.WithContext(c.Request.Context(), log)
		fields := requestLogFields(c, time.Since(start))

		status := c.Writer.Status()
		switch {
		// 5xx 表示服务端失败，4xx 通常表示调用方输入或授权状态问题。
		case status >= 500:
			reqLog.Error("http request completed", fields...)
		case status >= 400:
			reqLog.Warn("http request completed", fields...)
		default:
			reqLog.Info("http request completed", fields...)
		}
	}
}

func requestUserID(c *gin.Context) string {
	if userID, ok := auth.UserIDFromContext(c.Request.Context()); ok {
		return userID
	}
	return anonymousUserID
}
