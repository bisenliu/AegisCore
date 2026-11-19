package middleware

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/logger"
)

const (
	// HeaderRequestID 是调用方和网关传递请求关联 ID 的 HTTP header。
	HeaderRequestID = "X-Request-ID"

	// maxRequestIDLength 是允许透传外部请求 ID 的最大字符长度，超出将被丢弃并重新生成。
	maxRequestIDLength = 128
	// asciiSpace 是标准 ASCII 可见字符区间的下边界（空格字符）。
	asciiSpace = ' '
	// asciiDel 是 ASCII 控制字符删除符（DEL）。
	asciiDel = 0x7f
	// base36 是降级请求流水号格式化时采用的 36 进制基数（包含数字与小写字母）。
	base36 = 36
)

var requestIDFallbackCounter atomic.Uint64

// RequestID 为每个 HTTP 请求生成或透传请求 ID，并写入响应头和 request context。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := normalizeRequestID(c.GetHeader(HeaderRequestID))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Header(HeaderRequestID, requestID)
		c.Request = c.Request.WithContext(logger.WithRequestID(c.Request.Context(), requestID))
		c.Next()
	}
}

// normalizeRequestID 清理请求追踪编号中的首尾空格与换行等不可见字符，防止恶意伪造的编号污染排查日志。
func normalizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxRequestIDLength || containsRequestIDControlCharacter(value) {
		return ""
	}
	return value
}

// containsRequestIDControlCharacter 检查请求追踪编号中是否夹带了不可见的控制字符（如换行符或制表符）。
func containsRequestIDControlCharacter(value string) bool {
	for _, char := range value {
		if char < asciiSpace || char == asciiDel {
			return true
		}
	}
	return false
}

// newRequestID 生成一个全局唯一的请求流水号，用于串联单次请求的全链路处理日志。
func newRequestID() string {
	requestID, err := id.NewUUIDString()
	if err == nil {
		return requestID
	}
	return "req-" + strconv.FormatInt(time.Now().UnixNano(), base36) + "-" + strconv.FormatUint(requestIDFallbackCounter.Add(1), base36)
}
