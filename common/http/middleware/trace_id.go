package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
)

const (
	// HeaderTraceID 是请求接收并在响应中回传的 trace id 请求头。
	HeaderTraceID = "X-Trace-ID"
	// TraceIDKey 是 Gin context 中存储有效 trace id 的 key。
	TraceIDKey = "trace_id"
	// ContextKeyLogger 是 Gin context 中存储请求 logger 的 key。
	ContextKeyLogger = "logger"

	// DefaultMaxTraceIDLength 限制调用方传入的 trace id 进入日志和上下文前的最大长度。
	DefaultMaxTraceIDLength = 128
)

// TraceIDOptions 配置 trace-id 请求头传播、长度限制和校验规则。
type TraceIDOptions struct {
	HeaderName string
	MaxLength  int
	Validate   func(string) bool
}

// TraceID 返回传播 X-Trace-ID 或生成 UUID trace id 的中间件。
func TraceID() gin.HandlerFunc {
	return TraceIDWithOptions(TraceIDOptions{})
}

// TraceIDWithOptions 返回使用自定义 trace-id 请求头和校验行为的中间件。
func TraceIDWithOptions(options TraceIDOptions) gin.HandlerFunc {
	headerName := options.HeaderName
	if headerName == "" {
		headerName = HeaderTraceID
	}
	maxLength := options.MaxLength
	if maxLength == 0 {
		// 0 是便于配置的默认值哨兵，不表示允许无限长度的 trace-id。
		maxLength = DefaultMaxTraceIDLength
	}
	return func(c *gin.Context) {
		traceID := strings.TrimSpace(c.GetHeader(headerName))
		if traceID == "" || len(traceID) > maxLength || (options.Validate != nil && !options.Validate(traceID)) {
			// 不可信 trace-id 会被重新生成，避免日志字段失控或继续传播无效关联数据。
			traceID = uuid.NewString()
		}
		ctx := logger.WithTraceID(c.Request.Context(), traceID)
		if base, ok := c.Get(ContextKeyLogger); ok {
			if log, ok := base.(*zap.Logger); ok {
				// 将请求 logger 保留到 context，使包级 logger helper 复用相同基础字段。
				ctx = logger.ToContext(ctx, log)
			}
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(TraceIDKey, traceID)
		c.Header(headerName, traceID)
		c.Next()
	}
}

func traceID(c *gin.Context) string {
	v, ok := c.Get(TraceIDKey)
	if !ok {
		return logger.TraceIDFromContext(c.Request.Context())
	}
	traceID, _ := v.(string)
	return traceID
}
