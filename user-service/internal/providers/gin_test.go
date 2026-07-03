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
	"github.com/stretchr/testify/require"
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

func TestNewGinEnginePassesThroughRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)

	var requestID string
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		var ok bool
		requestID, ok = commonmiddleware.RequestIDFromContext(c.Request.Context())
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
		requestID, ok = commonmiddleware.RequestIDFromContext(c.Request.Context())
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
		requestID, ok = commonmiddleware.RequestIDFromContext(c.Request.Context())
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

func TestNewGinEngineSkipsHealthProbeTracing(t *testing.T) {
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

	require.False(t, spanContext.TraceID().IsValid())
	require.False(t, spanContext.SpanID().IsValid())
}

func TestNewGinEngineSkipsMetricsTracingWhenEnabled(t *testing.T) {
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

	require.False(t, spanContext.TraceID().IsValid())
	require.False(t, spanContext.SpanID().IsValid())
}

func TestNewGinEngineRecordsHTTPServerMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, Metrics: metricsProvider})
	require.NoError(t, err)
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	metric := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/users/:user_id",
		commonmetrics.LabelStatusClass: "2xx",
	})
	require.Equal(t, float64(1), metric.GetCounter().GetValue())
	duration := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_request_duration_seconds"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/users/:user_id",
		commonmetrics.LabelStatusClass: "2xx",
	})
	require.Equal(t, uint64(1), duration.GetHistogram().GetSampleCount())
	assertGinMetricFamilyMissingLabelValue(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), "/api/v1/users/123")
}

func TestNewGinEngineSkipsSuccessfulRuntimeEndpointMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, Metrics: metricsProvider})
	require.NoError(t, err)
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
	require.NoError(t, err)

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	metric := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "__unmatched__",
		commonmetrics.LabelStatusClass: "4xx",
	})
	require.Equal(t, float64(1), metric.GetCounter().GetValue())
	assertGinMetricFamilyMissingLabelValue(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), "/api/v1/users/123")
}

func TestNewGinEngineRecordsPanicHTTPServerMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, Metrics: metricsProvider})
	require.NoError(t, err)
	engine.GET("/api/v1/panic", func(_ *gin.Context) {
		panic("metrics panic test")
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	metric := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/panic",
		commonmetrics.LabelStatusClass: "5xx",
	})
	require.Equal(t, float64(1), metric.GetCounter().GetValue())
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
	span := endedGinSpan(t, recorder)
	require.Equal(t, codes.Error, span.Status().Code)
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
	span := endedGinSpan(t, recorder)
	require.Equal(t, codes.Unset, span.Status().Code)
}

func TestNewGinEngineRecordsPanicSpanError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)
	engine.GET("/api/v1/panic", func(_ *gin.Context) {
		panic("route boom password token")
	})

	recorderHTTP := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil)
	engine.ServeHTTP(recorderHTTP, request)

	require.Equal(t, http.StatusInternalServerError, recorderHTTP.Code)
	var body struct {
		Success bool                `json:"success"`
		Code    contracterrors.Code `json:"code"`
	}
	require.NoError(t, json.NewDecoder(recorderHTTP.Body).Decode(&body))
	require.False(t, body.Success)
	require.Equal(t, contracterrors.CodeInternalError, body.Code)
	span := endedGinSpan(t, recorder)
	require.Equal(t, codes.Error, span.Status().Code)
	require.True(t, spanHasEvent(span, "exception"), "events=%#v", span.Events())
}

func TestNewGinEngineSkipsSuccessfulHealthProbeErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: provider})
	require.NoError(t, err)
	engine.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))

	require.Empty(t, recorder.Ended())
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
		require.Equal(t, "PATCH route not found", got)
		unmatchedNames[got] = struct{}{}
	}
	require.Len(t, unmatchedNames, 1)
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
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
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
	require.NoError(t, err)
	return provider
}

func gatherGinMetricFamily(t *testing.T, provider *commonmetrics.Provider, name string) *dto.MetricFamily {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	require.NoError(t, err)
	var found *dto.MetricFamily
	for _, family := range families {
		if family.GetName() == name {
			found = family
			break
		}
	}
	require.NotNil(t, found, "metric family %q not found", name)
	return found
}

func findGinMetricByLabels(t *testing.T, family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	t.Helper()
	var found *dto.Metric
	for _, metric := range family.GetMetric() {
		if ginMetricHasLabels(metric, labels) {
			found = metric
			break
		}
	}
	require.NotNil(t, found, "metric family %q missing labels %#v", family.GetName(), labels)
	return found
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
	require.NoError(t, err)
	for _, family := range families {
		require.NotEqual(t, name, family.GetName())
	}
}

func assertGinMetricFamilyMissingLabelValue(t *testing.T, family *dto.MetricFamily, value string) {
	t.Helper()
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			require.NotEqual(t, value, label.GetValue(), "metric family %q has forbidden label value", family.GetName())
		}
	}
}

func endedGinSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	require.Len(t, spans, 1)
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
