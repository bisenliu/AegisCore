package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/http/response"
)

// RequestBodyLimit 创建入站请求体字节上限 middleware。
// 已知 Content-Length 超限时立即拒绝；未知长度请求由 MaxBytesReader 在读取时强制边界。
func RequestBodyLimit(maxBytes int64) (gin.HandlerFunc, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("request body max bytes must be > 0")
	}
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			response.PayloadTooLarge(c)
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}, nil
}
