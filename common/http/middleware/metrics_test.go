package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

func TestHTTPServerMetricsDisabledProviderHasNoSideEffects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, false)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{Provider: provider}))
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	if provider.Enabled() {
		t.Fatal("provider is enabled")
	}
	if provider.Gatherer() != nil {
		t.Fatal("disabled provider has gatherer")
	}
}

func TestHTTPServerMetricsRecordsRequestCounterAndDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{Provider: provider}))
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	requestMetric := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerRequestsMetricName), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/users/:user_id",
		commonmetrics.LabelStatusClass: "2xx",
	})
	if got := requestMetric.GetCounter().GetValue(); got != 1 {
		t.Fatalf("request counter = %v, want 1", got)
	}

	durationMetric := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerDurationMetricName), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/users/:user_id",
		commonmetrics.LabelStatusClass: "2xx",
	})
	if got := durationMetric.GetHistogram().GetSampleCount(); got != 1 {
		t.Fatalf("duration sample count = %d, want 1", got)
	}
	assertHTTPMiddlewareFamilyMissingLabelValue(t, gatherHTTPMiddlewareFamily(t, provider, httpServerRequestsMetricName), "/api/v1/users/123")
}

func TestHTTPServerMetricsRecordsServerErrorStatusClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{Provider: provider}))
	engine.GET("/api/v1/fail", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/fail", nil))

	metric := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerRequestsMetricName), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/fail",
		commonmetrics.LabelStatusClass: "5xx",
	})
	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Fatalf("server error counter = %v, want 1", got)
	}
}

func TestHTTPServerMetricsUsesFallbackForUnmatchedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{Provider: provider}))

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))

	metric := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerRequestsMetricName), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       defaultHTTPMetricsRouteFallback,
		commonmetrics.LabelStatusClass: "4xx",
	})
	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Fatalf("unmatched counter = %v, want 1", got)
	}
	assertHTTPMiddlewareFamilyMissingLabelValue(t, gatherHTTPMiddlewareFamily(t, provider, httpServerRequestsMetricName), "/api/v1/users/123")
}

func TestHTTPServerMetricsTracksInFlightRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{Provider: provider}))
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	engine.GET("/api/v1/slow/:user_id", func(c *gin.Context) {
		close(started)
		<-release
		c.Status(http.StatusNoContent)
	})

	go func() {
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/slow/123", nil))
		close(done)
	}()
	<-started

	inFlightMetric := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerInFlightRequestsName), map[string]string{
		commonmetrics.LabelMethod: http.MethodGet,
		commonmetrics.LabelRoute:  "/api/v1/slow/:user_id",
	})
	if got := inFlightMetric.GetGauge().GetValue(); got != 1 {
		t.Fatalf("in-flight gauge = %v, want 1", got)
	}

	close(release)
	<-done

	inFlightMetric = findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerInFlightRequestsName), map[string]string{
		commonmetrics.LabelMethod: http.MethodGet,
		commonmetrics.LabelRoute:  "/api/v1/slow/:user_id",
	})
	if got := inFlightMetric.GetGauge().GetValue(); got != 0 {
		t.Fatalf("in-flight gauge after completion = %v, want 0", got)
	}
}

func TestHTTPServerMetricsSkipResultFiltersSuccessfulRuntimeEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{
		Provider: provider,
		SkipResult: func(c *gin.Context) bool {
			return c.Writer.Status() < http.StatusBadRequest && c.FullPath() == "/runtime"
		},
	}))
	engine.GET("/runtime", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	engine.GET("/runtime-fail", func(c *gin.Context) {
		c.Status(http.StatusServiceUnavailable)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/runtime", nil))
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/runtime-fail", nil))

	requests := gatherHTTPMiddlewareFamily(t, provider, httpServerRequestsMetricName)
	assertHTTPMiddlewareFamilyMissingLabelValue(t, requests, "/runtime")
	metric := findMetricByLabels(t, requests, map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/runtime-fail",
		commonmetrics.LabelStatusClass: "5xx",
	})
	if got := metric.GetCounter().GetValue(); got != 1 {
		t.Fatalf("runtime failure counter = %v, want 1", got)
	}
	assertHTTPMiddlewareFamilyMissingLabelValue(t, gatherHTTPMiddlewareFamily(t, provider, httpServerInFlightRequestsName), "/runtime")
}

func TestHTTPServerMetricsSkipFiltersBeforeInFlight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{
		Provider: provider,
		Skip: func(c *gin.Context) bool {
			return c.Request.URL.Path == "/metrics"
		},
	}))
	engine.GET("/metrics", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	assertHTTPMiddlewareFamilyMissing(t, provider, httpServerRequestsMetricName)
	assertHTTPMiddlewareFamilyMissing(t, provider, httpServerDurationMetricName)
	assertHTTPMiddlewareFamilyMissing(t, provider, httpServerInFlightRequestsName)
}

func TestHTTPServerMetricsDuplicateConstructionDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{Provider: provider}))
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{Provider: provider}))
	engine.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
}

func newHTTPMiddlewareMetricsProvider(t *testing.T, enabled bool) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: enabled, Path: "/metrics"},
		ServiceName: "aegiscore-common-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return provider
}

func gatherHTTPMiddlewareFamily(t *testing.T, provider *commonmetrics.Provider, name string) *dto.MetricFamily {
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

func findMetricByLabels(t *testing.T, family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	t.Helper()
	for _, metric := range family.GetMetric() {
		if metricHasLabels(metric, labels) {
			return metric
		}
	}
	t.Fatalf("metric family %q missing labels %#v", family.GetName(), labels)
	return nil
}

func metricHasLabels(metric *dto.Metric, labels map[string]string) bool {
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

func assertHTTPMiddlewareFamilyMissing(t *testing.T, provider *commonmetrics.Provider, name string) {
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

func assertHTTPMiddlewareFamilyMissingLabelValue(t *testing.T, family *dto.MetricFamily, value string) {
	t.Helper()
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetValue() == value {
				t.Fatalf("metric family %q has forbidden label value %q", family.GetName(), value)
			}
		}
	}
}
