package middleware

import (
	"time"

	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/netutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.WithContext(log, c.Request.Context()).Info("http request completed",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", netutil.GetClientIP(c)),
		)
	}
}
