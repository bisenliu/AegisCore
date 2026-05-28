package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.InfoContext(c.Request.Context(), "http request completed",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
			slog.String("request_id", requestID(c)),
		)
	}
}

func requestID(c *gin.Context) string {
	v, ok := c.Get(RequestIDKey)
	if !ok {
		return ""
	}
	requestID, _ := v.(string)
	return requestID
}
