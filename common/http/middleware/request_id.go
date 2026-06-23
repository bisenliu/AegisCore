package middleware

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/runtime/id"
)

const (
	// HeaderRequestID 是调用方和网关传递请求关联 ID 的 HTTP header。
	HeaderRequestID = "X-Request-ID"
	// RequestIDField 是结构化日志中记录请求关联 ID 的字段名。
	RequestIDField = "request_id"
)

const maxRequestIDLength = 128

type requestIDContextKey struct{}

var requestIDFallbackCounter atomic.Uint64

// RequestID 为每个 HTTP 请求生成或透传请求 ID，并写入响应头和 request context。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := normalizeRequestID(c.GetHeader(HeaderRequestID))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Header(HeaderRequestID, requestID)
		c.Request = c.Request.WithContext(WithRequestID(c.Request.Context(), requestID))
		c.Next()
	}
}

// WithRequestID 返回携带 request ID 的 context。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext 从 context 中读取 request ID。
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	requestID, ok := ctx.Value(requestIDContextKey{}).(string)
	return requestID, ok && requestID != ""
}

func normalizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxRequestIDLength || containsRequestIDControlCharacter(value) {
		return ""
	}
	return value
}

func containsRequestIDControlCharacter(value string) bool {
	for _, char := range value {
		if char < ' ' || char == 0x7f {
			return true
		}
	}
	return false
}

func newRequestID() string {
	requestID, err := id.NewUUIDString()
	if err == nil {
		return requestID
	}
	return "req-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(requestIDFallbackCounter.Add(1), 36)
}
