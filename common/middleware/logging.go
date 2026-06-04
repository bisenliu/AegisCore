package middleware

import (
	"time"

	"github.com/aegiscore/common/auth"
	"github.com/aegiscore/common/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const anonymousUserID = "anonymous"

func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		reqLog := logger.WithContext(log, c.Request.Context())
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
			zap.String(auth.UserIDKey, requestUserID(c)),
		}

		status := c.Writer.Status()
		switch {
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
