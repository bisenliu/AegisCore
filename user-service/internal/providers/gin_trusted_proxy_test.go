package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNewGinEngineDefaultsToNotTrustingForwardedClientIP(t *testing.T) {
	clientIP := newGinClientIPFromRequest(t, nil, "192.0.2.10:12345", map[string]string{
		"X-Forwarded-For": "203.0.113.50",
		"X-Real-IP":       "203.0.113.60",
	})

	require.Equal(t, "192.0.2.10", clientIP)
}

func TestNewGinEngineTrustsForwardedClientIPFromConfiguredProxy(t *testing.T) {
	clientIP := newGinClientIPFromRequest(t, []string{"192.0.2.10"}, "192.0.2.10:12345", map[string]string{
		"X-Forwarded-For": "203.0.113.50",
		"X-Real-IP":       "203.0.113.60",
	})

	require.Equal(t, "203.0.113.50", clientIP)
}

func TestNewGinEngineIgnoresForwardedClientIPFromUntrustedPeer(t *testing.T) {
	clientIP := newGinClientIPFromRequest(t, []string{"198.51.100.10"}, "192.0.2.10:12345", map[string]string{
		"X-Forwarded-For": "203.0.113.50",
		"X-Real-IP":       "203.0.113.60",
	})

	require.Equal(t, "192.0.2.10", clientIP)
}

func newGinClientIPFromRequest(t *testing.T, trustedProxies []string, remoteAddr string, headers map[string]string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Server.HTTP.TrustedProxies = trustedProxies
	traceProvider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider})
	require.NoError(t, err)

	var clientIP string
	engine.GET("/client-ip", func(c *gin.Context) {
		clientIP = c.ClientIP()
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	request.RemoteAddr = remoteAddr
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	engine.ServeHTTP(httptest.NewRecorder(), request)

	return clientIP
}
