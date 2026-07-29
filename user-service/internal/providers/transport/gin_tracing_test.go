package transport

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	commonresponse "github.com/aegiscore/common/http/response"
	"github.com/aegiscore/common/runtime/config"
)

func TestNewGinEngineCreatesOTelServerSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var spanContext trace.SpanContext
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, spanContext.TraceID().IsValid())
	require.True(t, spanContext.SpanID().IsValid())
}

func TestNewGinEngineExtractsTraceparent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var spanContext trace.SpanContext
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set("traceparent", "00-00112233445566778899aabbccddeeff-0102030405060708-01")
	engine.ServeHTTP(httptest.NewRecorder(), request)

	require.Equal(t, "00112233445566778899aabbccddeeff", spanContext.TraceID().String())
}

func TestNewGinEngineTracesHealthProbeWithoutRawPathFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var spanContext trace.SpanContext
	engine.GET("/livez", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))

	require.True(t, spanContext.TraceID().IsValid())
	require.True(t, spanContext.SpanID().IsValid())
}

func TestNewGinEngineTracesMetricsWithoutRawPathFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var spanContext trace.SpanContext
	engine.GET("/metrics", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.True(t, spanContext.TraceID().IsValid())
	require.True(t, spanContext.SpanID().IsValid())
}

func TestNewGinEngineMarksServerErrorSpanStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)
	failResponse := commonresponse.Fail
	engine.GET("/api/v1/fail", func(c *gin.Context) {
		failResponse(c, errors.New("database password token"))
	})

	recorderHTTP := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/fail", nil)
	engine.ServeHTTP(recorderHTTP, request)

	require.Equal(t, http.StatusInternalServerError, recorderHTTP.Code)
	span := endedGinSpan(t, provider, recorder)
	require.Equal(t, tracepb.Status_STATUS_CODE_ERROR, span.GetStatus().GetCode())
}

func TestNewGinEngineDoesNotMarkClientErrorSpanStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)
	engine.GET("/api/v1/bad-request", func(c *gin.Context) {
		commonresponse.BadRequest(c, "请求格式错误")
	})

	recorderHTTP := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bad-request", nil)
	engine.ServeHTTP(recorderHTTP, request)

	require.Equal(t, http.StatusBadRequest, recorderHTTP.Code)
	span := endedGinSpan(t, provider, recorder)
	require.Equal(t, tracepb.Status_STATUS_CODE_UNSET, span.GetStatus().GetCode())
}

func TestNewGinEngineKeepsSuccessfulHealthProbeSpanStatusUnset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)
	engine.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))

	span := endedGinSpan(t, provider, recorder)
	require.Equal(t, tracepb.Status_STATUS_CODE_UNSET, span.GetStatus().GetCode())
}

func TestHTTPServerSpanName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		require.Equal(t, "GET /api/v1/users/:user_id", httpServerSpanName(c))
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	unmatchedNames := make(map[string]struct{})
	for _, path := range []string{"/not-found/1", "/not-found/2"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPatch, path, nil)
		got := httpServerSpanName(c)
		require.Equal(t, "PATCH __unmatched__", got)
		unmatchedNames[got] = struct{}{}
	}
	require.Len(t, unmatchedNames, 1)
}

func TestNewGinEngineRenamesSpansWithLowCardinalityRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rawPath := "/api/v1/users/018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, rawPath, nil))

	span := endedGinSpan(t, provider, recorder)
	require.Equal(t, "GET /api/v1/users/:user_id", span.GetName())
	require.NotEqual(t, "GET "+rawPath, span.GetName())
}

func TestNewGinEngineRenamesUnmatchedSpanWithStableFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	rawPath := "/unknown/018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, rawPath, nil))

	span := endedGinSpan(t, provider, recorder)
	require.Equal(t, "GET __unmatched__", span.GetName())
	require.NotEqual(t, "GET "+rawPath, span.GetName())
}
