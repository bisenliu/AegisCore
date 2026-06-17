package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonresponse "github.com/aegiscore/common/http/response"
	"github.com/aegiscore/common/runtime/config"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

func TestSkipSuccessfulRuntimeEndpointLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	skip := skipSuccessfulRuntimeEndpointLog(config.MetricsConfig{Enabled: true, Path: "/metrics"})
	engine.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
		if !skip(c) {
			t.Fatal("skipSuccessfulRuntimeEndpointLog = false, want true")
		}
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.Status(http.StatusServiceUnavailable)
		if skip(c) {
			t.Fatal("skipSuccessfulRuntimeEndpointLog = true, want false")
		}
	})
	engine.GET("/metrics", func(c *gin.Context) {
		c.Status(http.StatusOK)
		if !skip(c) {
			t.Fatal("skipSuccessfulRuntimeEndpointLog = false, want true")
		}
	})
	engine.GET("/api/v1/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
		if skip(c) {
			t.Fatal("skipSuccessfulRuntimeEndpointLog = true, want false")
		}
	})

	for _, path := range []string{"/livez", "/readyz", "/metrics", "/api/v1/users"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, request)
	}
}

func TestNewGinEngineCreatesOTelServerSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	var spanContext trace.SpanContext
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if !spanContext.TraceID().IsValid() || !spanContext.SpanID().IsValid() {
		t.Fatalf("span context = %v, want valid trace and span IDs", spanContext)
	}
}

func TestNewGinEngineExtractsTraceparent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	var spanContext trace.SpanContext
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set("traceparent", "00-00112233445566778899aabbccddeeff-0102030405060708-01")
	engine.ServeHTTP(httptest.NewRecorder(), request)

	if got := spanContext.TraceID().String(); got != "00112233445566778899aabbccddeeff" {
		t.Fatalf("trace id = %q, want propagated traceparent trace id", got)
	}
}

func TestNewGinEngineSkipsHealthProbeTracing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	var spanContext trace.SpanContext
	engine.GET("/livez", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))

	if spanContext.TraceID().IsValid() || spanContext.SpanID().IsValid() {
		t.Fatalf("health probe span context = %v, want invalid because tracing is filtered", spanContext)
	}
}

func TestNewGinEngineSkipsMetricsTracingWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	var spanContext trace.SpanContext
	engine.GET("/metrics", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if spanContext.TraceID().IsValid() || spanContext.SpanID().IsValid() {
		t.Fatalf("metrics span context = %v, want invalid because tracing is filtered", spanContext)
	}
}

func TestNewGinEngineMarksServerErrorSpanStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/api/v1/fail", func(c *gin.Context) {
		commonresponse.Fail(c, errors.New("database password token"))
	})

	recorderHTTP := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/fail", nil)
	engine.ServeHTTP(recorderHTTP, request)

	if recorderHTTP.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorderHTTP.Code)
	}
	span := endedGinSpan(t, recorder)
	if got := span.Status().Code; got != codes.Error {
		t.Fatalf("span status = %s, want Error", got)
	}
}

func TestNewGinEngineDoesNotMarkClientErrorSpanStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/api/v1/bad-request", func(c *gin.Context) {
		commonresponse.BadRequest(c, "请求格式错误")
	})

	recorderHTTP := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bad-request", nil)
	engine.ServeHTTP(recorderHTTP, request)

	if recorderHTTP.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorderHTTP.Code)
	}
	span := endedGinSpan(t, recorder)
	if got := span.Status().Code; got != codes.Unset {
		t.Fatalf("span status = %s, want Unset", got)
	}
}

func TestNewGinEngineRecordsPanicSpanError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/api/v1/panic", func(_ *gin.Context) {
		panic("route boom password token")
	})

	recorderHTTP := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil)
	engine.ServeHTTP(recorderHTTP, request)

	if recorderHTTP.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorderHTTP.Code)
	}
	var body struct {
		Success bool                `json:"success"`
		Code    contracterrors.Code `json:"code"`
	}
	if err := json.NewDecoder(recorderHTTP.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Success || body.Code != contracterrors.CodeInternalError {
		t.Fatalf("body = %#v, want internal failure envelope", body)
	}
	span := endedGinSpan(t, recorder)
	if got := span.Status().Code; got != codes.Error {
		t.Fatalf("span status = %s, want Error", got)
	}
	if !spanHasEvent(span, "exception") {
		t.Fatalf("span events = %#v, want exception event", span.Events())
	}
}

func TestNewGinEngineSkipsSuccessfulHealthProbeErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))

	if spans := recorder.Ended(); len(spans) != 0 {
		t.Fatalf("ended spans = %d, want 0 for successful health probe", len(spans))
	}
}

func TestHTTPServerSpanName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		if got := httpServerSpanName(c); got != "GET /api/v1/users/:user_id" {
			t.Fatalf("span name = %q, want route template", got)
		}
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPatch, "/not-found", nil)
	if got := httpServerSpanName(c); got != "PATCH /not-found" {
		t.Fatalf("fallback span name = %q, want URL path", got)
	}
}

func ginTestConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{Name: "configured-user-service", Environment: "test"},
		Observability: config.ObservabilityConfig{
			Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, Exporter: "none"},
		},
	}
}

func newGinTestTracingProvider(t *testing.T, cfg *config.Config) *commontracing.Provider {
	t.Helper()
	provider, err := commontracing.NewProvider(context.Background(), commontracing.Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown tracing provider: %v", err)
		}
	})
	return provider
}

func newGinTestTracingProviderWithRecorder(t *testing.T, cfg *config.Config) (*commontracing.Provider, *tracetest.SpanRecorder) {
	t.Helper()
	provider := newGinTestTracingProvider(t, cfg)
	recorder := tracetest.NewSpanRecorder()
	provider.TracerProvider().RegisterSpanProcessor(recorder)
	return provider, recorder
}

func endedGinSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	return spans[0]
}

func spanHasEvent(span sdktrace.ReadOnlySpan, name string) bool {
	for _, event := range span.Events() {
		if event.Name == name {
			return true
		}
	}
	return false
}
