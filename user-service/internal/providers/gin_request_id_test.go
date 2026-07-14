package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	commonmiddleware "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/runtime/logger"
)

func TestNewGinEnginePassesThroughRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var requestID string
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		var ok bool
		requestID, ok = logger.RequestIDFromContext(c.Request.Context())
		require.True(t, ok)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set(commonmiddleware.HeaderRequestID, "client-request-123")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, "client-request-123", recorder.Header().Get(commonmiddleware.HeaderRequestID))
	require.Equal(t, "client-request-123", requestID)
}

func TestNewGinEngineGeneratesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var requestID string
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		var ok bool
		requestID, ok = logger.RequestIDFromContext(c.Request.Context())
		require.True(t, ok)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	responseRequestID := recorder.Header().Get(commonmiddleware.HeaderRequestID)
	require.NotEmpty(t, responseRequestID)
	require.Equal(t, responseRequestID, requestID)
}

func TestNewGinEngineKeepsTraceparentAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var spanContext trace.SpanContext
	var requestID string
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		var ok bool
		requestID, ok = logger.RequestIDFromContext(c.Request.Context())
		require.True(t, ok)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set("traceparent", "00-00112233445566778899aabbccddeeff-0102030405060708-01")
	request.Header.Set(commonmiddleware.HeaderRequestID, "client-request-123")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, "00112233445566778899aabbccddeeff", spanContext.TraceID().String())
	require.Equal(t, "client-request-123", requestID)
	require.Equal(t, "client-request-123", recorder.Header().Get(commonmiddleware.HeaderRequestID))
}
