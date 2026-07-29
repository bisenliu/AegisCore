package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
)

func TestSkipSuccessfulRuntimeEndpointLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	skip := skipSuccessfulRuntimeEndpointLog(config.MetricsConfig{Enabled: true, Path: "/metrics"})
	results := make(map[string]bool)
	engine.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
		results["/livez"] = skip(c)
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.Status(http.StatusServiceUnavailable)
		results["/readyz"] = skip(c)
	})
	engine.GET("/metrics", func(c *gin.Context) {
		c.Status(http.StatusOK)
		results["/metrics"] = skip(c)
	})
	engine.GET("/api/v1/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
		results["/api/v1/users"] = skip(c)
	})

	for _, path := range []string{"/livez", "/readyz", "/metrics", "/api/v1/users"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, request)
	}
	require.True(t, results["/livez"])
	require.False(t, results["/readyz"])
	require.True(t, results["/metrics"])
	require.False(t, results["/api/v1/users"])
}
