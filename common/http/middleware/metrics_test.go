package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

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

	require.False(t, provider.Enabled())
	require.Nil(t, provider.Gatherer())
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
	require.Equal(t, float64(1), requestMetric.GetCounter().GetValue())

	durationMetric := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerDurationMetricName), map[string]string{
		commonmetrics.LabelMethod:      http.MethodGet,
		commonmetrics.LabelRoute:       "/api/v1/users/:user_id",
		commonmetrics.LabelStatusClass: "2xx",
	})
	require.Equal(t, uint64(1), durationMetric.GetHistogram().GetSampleCount())
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
	require.Equal(t, float64(1), metric.GetCounter().GetValue())
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
	require.Equal(t, float64(1), metric.GetCounter().GetValue())
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
	require.Equal(t, float64(1), inFlightMetric.GetGauge().GetValue())

	close(release)
	<-done

	inFlightMetric = findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerInFlightRequestsName), map[string]string{
		commonmetrics.LabelMethod: http.MethodGet,
		commonmetrics.LabelRoute:  "/api/v1/slow/:user_id",
	})
	require.Equal(t, float64(0), inFlightMetric.GetGauge().GetValue())
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
	require.Equal(t, float64(1), metric.GetCounter().GetValue())
	inFlight := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerInFlightRequestsName), map[string]string{
		commonmetrics.LabelMethod: http.MethodGet,
		commonmetrics.LabelRoute:  "/runtime",
	})
	require.Zero(t, inFlight.GetGauge().GetValue())
}

func TestHTTPServerMetricsSkipResultDoesNotCorruptConcurrentInFlightGauge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newHTTPMiddlewareMetricsProvider(t, true)
	engine := gin.New()
	engine.Use(HTTPServerMetrics(HTTPMetricsOptions{
		Provider: provider,
		SkipResult: func(c *gin.Context) bool {
			return c.Writer.Status() < http.StatusBadRequest
		},
	}))
	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	slowDone := make(chan struct{})
	engine.GET("/runtime/:result", func(c *gin.Context) {
		if c.Param("result") == "slow" {
			close(slowStarted)
			<-releaseSlow
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusNoContent)
	})

	go func() {
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/runtime/slow", nil))
		close(slowDone)
	}()
	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("slow request did not start")
	}
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/runtime/filtered", nil))
	close(releaseSlow)
	select {
	case <-slowDone:
	case <-time.After(time.Second):
		t.Fatal("slow request did not finish")
	}

	inFlight := findMetricByLabels(t, gatherHTTPMiddlewareFamily(t, provider, httpServerInFlightRequestsName), map[string]string{
		commonmetrics.LabelMethod: http.MethodGet,
		commonmetrics.LabelRoute:  "/runtime/:result",
	})
	require.Zero(t, inFlight.GetGauge().GetValue())
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
	require.NoError(t, err)
	return provider
}

func gatherHTTPMiddlewareFamily(t *testing.T, provider *commonmetrics.Provider, name string) *dto.MetricFamily {
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

func findMetricByLabels(t *testing.T, family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	t.Helper()
	var found *dto.Metric
	for _, metric := range family.GetMetric() {
		if metricHasLabels(metric, labels) {
			found = metric
			break
		}
	}
	require.NotNil(t, found, "metric family %q missing labels %#v", family.GetName(), labels)
	return found
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
	require.NoError(t, err)
	for _, family := range families {
		require.NotEqual(t, name, family.GetName())
	}
}

func assertHTTPMiddlewareFamilyMissingLabelValue(t *testing.T, family *dto.MetricFamily, value string) {
	t.Helper()
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			require.NotEqual(t, value, label.GetValue(), "metric family %q has forbidden label value", family.GetName())
		}
	}
}
