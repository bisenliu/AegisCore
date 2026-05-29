package middleware

import (
	"github.com/aegiscore/common/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const TraceIDKey = "trace_id"

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.NewString()
		}
		ctx := logger.WithTraceID(c.Request.Context(), traceID)
		if base, ok := c.Get("logger"); ok {
			if log, ok := base.(*zap.Logger); ok {
				ctx = logger.ToContext(ctx, log)
			}
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(TraceIDKey, traceID)
		c.Header("X-Trace-ID", traceID)
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
