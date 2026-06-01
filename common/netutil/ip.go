package netutil

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	XForwardedFor = "X-Forwarded-For"
	XRealIP       = "X-Real-IP"
	XClientIP     = "X-Client-IP"
)

// GetClientIP returns the best available client IP from common proxy headers,
// falling back to Gin's configured client IP resolution.
func GetClientIP(c *gin.Context) string {
	if ip := firstForwardedIP(c.GetHeader(XForwardedFor)); ip != "" {
		return ip
	}

	if ip := strings.TrimSpace(c.GetHeader(XRealIP)); ip != "" {
		return ip
	}

	if ip := strings.TrimSpace(c.GetHeader(XClientIP)); ip != "" {
		return ip
	}

	return c.ClientIP()
}

func firstForwardedIP(value string) string {
	for _, candidate := range strings.Split(value, ",") {
		if ip := strings.TrimSpace(candidate); ip != "" {
			return ip
		}
	}

	return ""
}
