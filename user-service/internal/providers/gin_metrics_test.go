package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

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
