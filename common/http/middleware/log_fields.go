package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/aegiscore/common/security/auth"
)

var requestLogFieldPool = sync.Pool{
	New: func() any {
		fields := make([]zap.Field, 0, 9)
		return &fields
	},
}

// requestLogFields 返回 HTTP access log 的标准字段。
func requestLogFields(c *gin.Context, latency time.Duration) *[]zap.Field {
	fieldsRef := requestLogFieldPool.Get().(*[]zap.Field)
	fields := (*fieldsRef)[:0]
	fields = append(fields,
		zap.String("method", c.Request.Method),
		zap.String("path", requestPath(c)),
		zap.Int("status", c.Writer.Status()),
		zap.Int64("latency_ms", latency.Milliseconds()),
		zap.String("client_ip", c.ClientIP()),
		zap.String(auth.UserIDKey, requestUserID(c)),
	)
	*fieldsRef = fields
	return fieldsRef
}

// releaseRequestLogFields 清空并归还 HTTP access log 字段切片。
func releaseRequestLogFields(fieldsRef *[]zap.Field) {
	fields := *fieldsRef
	for index := range fields {
		fields[index] = zap.Field{}
	}
	*fieldsRef = fields[:0]
	requestLogFieldPool.Put(fieldsRef)
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
