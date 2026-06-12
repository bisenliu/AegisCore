package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/aegiscore/common/security/auth"
)

// requestLogFields 返回 HTTP access log 的标准字段。
func requestLogFields(c *gin.Context, latency time.Duration) []zap.Field {
	return []zap.Field{
		zap.String("method", c.Request.Method),
		zap.String("path", requestPath(c)),
		zap.Int("status", c.Writer.Status()),
		zap.Int64("latency_ms", latency.Milliseconds()),
		zap.String("client_ip", c.ClientIP()),
		zap.String(auth.UserIDKey, requestUserID(c)),
	}
}

// authFailureLogFields 返回认证失败安全事件日志的请求上下文字段。
func authFailureLogFields(c *gin.Context) []zap.Field {
	return []zap.Field{
		zap.String("method", c.Request.Method),
		zap.String("path", requestPath(c)),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.GetHeader("User-Agent")),
	}
}

func requestPath(c *gin.Context) string {
	if path := c.FullPath(); path != "" {
		return path
	}
	return c.Request.URL.Path
}
