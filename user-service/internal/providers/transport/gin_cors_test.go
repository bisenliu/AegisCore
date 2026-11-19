package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGinEngineCORSDisabledBypassesMiddleware(t *testing.T) {
	cfg := ginTestConfig()
	traceProvider := newGinTestTracingProvider(t, cfg)
	settings := ginTestHTTPSettings()
	settings.CORS.Enabled = false

	engine, err := NewGinEngine(GinParams{
		Config: cfg,
		Trace:  traceProvider,
		HTTP:   settings,
	})
	require.NoError(t, err)

	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://attacker.com")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	// 当禁用 CORS 时，Gin 默认处理 OPTIONS 请求，不会注入 Access-Control-Allow-Origin
	require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestGinEngineCORSEnabledAppliesConfiguredOrigins(t *testing.T) {
	cfg := ginTestConfig()
	traceProvider := newGinTestTracingProvider(t, cfg)
	settings := ginTestHTTPSettings()
	settings.CORS.Enabled = true
	settings.CORS.AllowedOrigins = []string{"https://admin.aegiscore.com"}
	settings.CORS.AllowedMethods = []string{http.MethodGet, http.MethodPost}

	engine, err := NewGinEngine(GinParams{
		Config: cfg,
		Trace:  traceProvider,
		HTTP:   settings,
	})
	require.NoError(t, err)

	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://admin.aegiscore.com")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "https://admin.aegiscore.com", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), "GET")
	require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), "POST")
}
