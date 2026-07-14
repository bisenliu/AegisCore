package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

func TestNewGinEngineDoesNotTrustForwardedClientIP(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{Name: "configured-user-service", Environment: "test"}}
	traceProvider, err := commontracing.NewProvider(context.Background(), commontracing.Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, traceProvider.Shutdown(context.Background())) })
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider})
	require.NoError(t, err)

	var clientIP string
	engine.GET("/client-ip", func(c *gin.Context) {
		clientIP = c.ClientIP()
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.50")
	request.Header.Set("X-Real-IP", "203.0.113.60")
	engine.ServeHTTP(httptest.NewRecorder(), request)

	require.Equal(t, "192.0.2.10", clientIP)
}
