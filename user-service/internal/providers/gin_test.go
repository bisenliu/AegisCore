package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonmiddleware "github.com/aegiscore/common/http/middleware"
	commonresponse "github.com/aegiscore/common/http/response"
	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
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

func TestNewGinEnginePassesThroughRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	var requestID string
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		var ok bool
		requestID, ok = commonmiddleware.RequestIDFromContext(c.Request.Context())
		if !ok {
			t.Fatal("request id missing from context")
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set(commonmiddleware.HeaderRequestID, "client-request-123")
	engine.ServeHTTP(recorder, request)

	if got := recorder.Header().Get(commonmiddleware.HeaderRequestID); got != "client-request-123" {
		t.Fatalf("response request id = %q, want client-request-123", got)
	}
	if requestID != "client-request-123" {
		t.Fatalf("context request id = %q, want client-request-123", requestID)
	}
}

func TestNewGinEngineGeneratesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	var requestID string
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		var ok bool
		requestID, ok = commonmiddleware.RequestIDFromContext(c.Request.Context())
		if !ok {
			t.Fatal("request id missing from context")
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	responseRequestID := recorder.Header().Get(commonmiddleware.HeaderRequestID)
	if responseRequestID == "" {
		t.Fatal("response request id is empty")
	}
	if requestID != responseRequestID {
		t.Fatalf("context request id = %q, want response request id %q", requestID, responseRequestID)
	}
}

func TestNewGinEngineKeepsTraceparentAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	var spanContext trace.SpanContext
	var requestID string
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		spanContext = trace.SpanContextFromContext(c.Request.Context())
		var ok bool
		requestID, ok = commonmiddleware.RequestIDFromContext(c.Request.Context())
		if !ok {
			t.Fatal("request id missing from context")
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set("traceparent", "00-00112233445566778899aabbccddeeff-0102030405060708-01")
	request.Header.Set(commonmiddleware.HeaderRequestID, "client-request-123")
	engine.ServeHTTP(recorder, request)

	if got := spanContext.TraceID().String(); got != "00112233445566778899aabbccddeeff" {
		t.Fatalf("trace id = %q, want propagated traceparent trace id", got)
	}
	if requestID != "client-request-123" {
		t.Fatalf("context request id = %q, want client-request-123", requestID)
	}
	if got := recorder.Header().Get(commonmiddleware.HeaderRequestID); got != "client-request-123" {
		t.Fatalf("response request id = %q, want client-request-123", got)
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

func TestNewGinEngineRecordsHTTPServerMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, Metrics: metricsProvider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	metric := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/users/:user_id",
		commonmetrics.LabelStatusClass: "2xx",
	})
	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Fatalf("request counter = %v, want 1", got)
	}
	duration := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_request_duration_seconds"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/users/:user_id",
		commonmetrics.LabelStatusClass: "2xx",
	})
	if got := duration.GetHistogram().GetSampleCount(); got != 1 {
		t.Fatalf("duration sample count = %d, want 1", got)
	}
	assertGinMetricFamilyMissingLabelValue(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), "/api/v1/users/123")
}

func TestNewGinEngineSkipsSuccessfulRuntimeEndpointMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, Metrics: metricsProvider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	for _, path := range []string{"/livez", "/readyz", "/startupz", "/metrics"} {
		path := path
		engine.GET(path, func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	assertGinMetricFamilyMissing(t, metricsProvider, "http_server_requests_total")
	assertGinMetricFamilyMissing(t, metricsProvider, "http_server_request_duration_seconds")
	assertGinMetricFamilyMissing(t, metricsProvider, "http_server_in_flight_requests")
}

func TestNewGinEngineRecordsUnmatchedRouteWithStableFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, Metrics: metricsProvider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	metric := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "__unmatched__",
		commonmetrics.LabelStatusClass: "4xx",
	})
	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Fatalf("unmatched counter = %v, want 1", got)
	}
	assertGinMetricFamilyMissingLabelValue(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), "/api/v1/users/123")
}

func TestNewGinEngineRecordsPanicHTTPServerMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, Metrics: metricsProvider})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/api/v1/panic", func(_ *gin.Context) {
		panic("metrics panic test")
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}

	metric := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/panic",
		commonmetrics.LabelStatusClass: "5xx",
	})
	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Fatalf("panic counter = %v, want 1", got)
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

	unmatchedNames := make(map[string]struct{})
	for _, path := range []string{"/not-found/1", "/not-found/2"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPatch, path, nil)
		got := httpServerSpanName(c)
		if got != "PATCH route not found" {
			t.Fatalf("fallback span name = %q, want stable unmatched route name", got)
		}
		unmatchedNames[got] = struct{}{}
	}
	if len(unmatchedNames) != 1 {
		t.Fatalf("unmatched span names = %v, want single low-cardinality fallback", unmatchedNames)
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

func newGinTestMetricsProvider(t *testing.T, cfg *config.Config) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      cfg.Observability.Metrics,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return provider
}

func gatherGinMetricFamily(t *testing.T, provider *commonmetrics.Provider, name string) *dto.MetricFamily {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("metric family %q not found", name)
	return nil
}

func findGinMetricByLabels(t *testing.T, family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	t.Helper()
	for _, metric := range family.GetMetric() {
		if ginMetricHasLabels(metric, labels) {
			return metric
		}
	}
	t.Fatalf("metric family %q missing labels %#v", family.GetName(), labels)
	return nil
}

func ginMetricHasLabels(metric *dto.Metric, labels map[string]string) bool {
	for key, want := range labels {
		found := false
		for _, label := range metric.GetLabel() {
			if label.GetName() == key {
				if label.GetValue() != want {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func assertGinMetricFamilyMissing(t *testing.T, provider *commonmetrics.Provider, name string) {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name {
			t.Fatalf("metric family %q exists, want missing", name)
		}
	}
}

func assertGinMetricFamilyMissingLabelValue(t *testing.T, family *dto.MetricFamily, value string) {
	t.Helper()
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetValue() == value {
				t.Fatalf("metric family %q has forbidden label value %q", family.GetName(), value)
			}
		}
	}
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
