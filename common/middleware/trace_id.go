package middleware

import (
	"strings"

	"github.com/aegiscore/common/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
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

type TraceIDOptions struct {
	HeaderName string
	MaxLength  int
	Validate   func(string) bool
}

func TraceID() gin.HandlerFunc {
	return TraceIDWithOptions(TraceIDOptions{})
}

func TraceIDWithOptions(options TraceIDOptions) gin.HandlerFunc {
	headerName := options.HeaderName
	if headerName == "" {
		headerName = HeaderTraceID
	}
	maxLength := options.MaxLength
	if maxLength == 0 {
		maxLength = DefaultMaxTraceIDLength
	}
	return func(c *gin.Context) {
		traceID := strings.TrimSpace(c.GetHeader(headerName))
		if traceID == "" || len(traceID) > maxLength || (options.Validate != nil && !options.Validate(traceID)) {
			traceID = uuid.NewString()
		}
		ctx := logger.WithTraceID(c.Request.Context(), traceID)
		if base, ok := c.Get(ContextKeyLogger); ok {
			if log, ok := base.(*zap.Logger); ok {
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
