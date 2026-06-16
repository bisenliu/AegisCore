package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/aegiscore/common/runtime/config"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

func TestSkipSuccessfulHealthProbeLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
		if !skipSuccessfulHealthProbeLog(c) {
			t.Fatal("skipSuccessfulHealthProbeLog = false, want true")
		}
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.Status(http.StatusServiceUnavailable)
		if skipSuccessfulHealthProbeLog(c) {
			t.Fatal("skipSuccessfulHealthProbeLog = true, want false")
		}
	})
	engine.GET("/api/v1/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
		if skipSuccessfulHealthProbeLog(c) {
			t.Fatal("skipSuccessfulHealthProbeLog = true, want false")
		}
	})

	for _, path := range []string{"/livez", "/readyz", "/api/v1/users"} {
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
