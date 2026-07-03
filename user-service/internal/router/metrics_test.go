package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

func TestRegisterMetricsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("disabled does not register route", func(t *testing.T) {
		engine := gin.New()
		provider := newRouterTestMetricsProvider(t, false, "/metrics")
		err := registerMetricsRoute(engine, MetricsRouteParams{Config: metricsRouteConfig(false, "/metrics"), Provider: provider})
		require.NoError(t, err)
		recorder := executeMetricsRequest(engine, "/metrics")
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("enabled exposes prometheus text", func(t *testing.T) {
		engine := gin.New()
		provider := newRouterTestMetricsProvider(t, true, "/metrics")
		registerRouterTestCounter(t, provider)
		err := registerMetricsRoute(engine, MetricsRouteParams{Config: metricsRouteConfig(true, "/metrics"), Provider: provider})
		require.NoError(t, err)
		recorder := executeMetricsRequest(engine, "/metrics")
		require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
		require.Contains(t, recorder.Header().Get("Content-Type"), "text/plain")
		require.Contains(t, recorder.Body.String(), "aegiscore_router_test_total")
	})

	t.Run("custom path does not also expose default path", func(t *testing.T) {
		engine := gin.New()
		provider := newRouterTestMetricsProvider(t, true, "/internal/metrics")
		registerRouterTestCounter(t, provider)
		err := registerMetricsRoute(engine, MetricsRouteParams{Config: metricsRouteConfig(true, "/internal/metrics"), Provider: provider})
		require.NoError(t, err)
		recorder := executeMetricsRequest(engine, "/metrics")
		require.Equal(t, http.StatusNotFound, recorder.Code)
		recorder = executeMetricsRequest(engine, "/internal/metrics")
		require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
	})

	t.Run("request context cancels context-aware collector", func(t *testing.T) {
		engine := gin.New()
		provider := newRouterTestMetricsProvider(t, true, "/metrics")
		collector := newRouterBlockingCollector()
		require.NoError(t, provider.Register(collector))
		require.NoError(t, registerMetricsRoute(engine, MetricsRouteParams{Config: metricsRouteConfig(true, "/metrics"), Provider: provider}))

		ctx, cancel := context.WithCancel(context.Background())
		request := httptest.NewRequest(http.MethodGet, "/metrics", nil).WithContext(ctx)
		done := make(chan struct{})
		go func() {
			engine.ServeHTTP(httptest.NewRecorder(), request)
			close(done)
		}()

		require.Eventually(t, func() bool {
			select {
			case <-collector.started:
				return true
			default:
				return false
			}
		}, time.Second, 10*time.Millisecond)
		cancel()

		require.Eventually(t, func() bool {
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, time.Second, 10*time.Millisecond)
		require.ErrorIs(t, collector.ctxErr(), context.Canceled)
	})
}

func TestRegisterMetricsRouteRejectsInvalidPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := newRouterTestMetricsProvider(t, true, "/metrics")
	tests := []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "relative", path: "metrics"},
		{name: "wildcard", path: "/metrics/*any"},
		{name: "parameter", path: "/metrics/:tenant"},
		{name: "root", path: "/"},
		{name: "health", path: "/livez"},
		{name: "api", path: "/api/v1/metrics"},
		{name: "openapi", path: "/openapi/metrics"},
		{name: "docs", path: "/docs"},
		{name: "api docs", path: "/api-docs"},
		{name: "pprof default", path: "/debug/pprof"},
		{name: "pprof custom", path: "/internal/debug/pprof/metrics"},
		{name: "unclean", path: "/internal/../metrics"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			err := registerMetricsRoute(engine, MetricsRouteParams{
				Config:   metricsRouteConfig(true, tt.path),
				Pprof:    config.PprofConfig{Enabled: true, BasePath: "/internal/debug/pprof"},
				Provider: provider,
			})
			require.ErrorIs(t, err, ErrInvalidMetricsPath)
		})
	}
}

func TestIsLowNoiseRuntimePath(t *testing.T) {
	cfg := metricsRouteConfig(true, "/metrics")
	require.True(t, IsLowNoiseRuntimePath("/livez", cfg))
	require.True(t, IsLowNoiseRuntimePath("/metrics", cfg))
	require.False(t, IsLowNoiseRuntimePath("/api/v1/users", cfg))
	require.False(t, IsLowNoiseRuntimePath("/metrics", metricsRouteConfig(false, "/metrics")))
}

func newRouterTestMetricsProvider(t *testing.T, enabled bool, metricsPath string) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      metricsRouteConfig(enabled, metricsPath),
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	require.NoError(t, err)
	return provider
}

func registerRouterTestCounter(t *testing.T, provider *commonmetrics.Provider) {
	t.Helper()
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_router_test_total",
		Help: "Router metrics endpoint test counter.",
	})
	counter.Inc()
	require.NoError(t, provider.Register(counter))
}

func metricsRouteConfig(enabled bool, metricsPath string) config.MetricsConfig {
	return config.MetricsConfig{Enabled: enabled, Path: metricsPath}
}

func executeMetricsRequest(engine *gin.Engine, requestPath string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, requestPath, nil)
	engine.ServeHTTP(recorder, request)
	return recorder
}

type routerBlockingCollector struct {
	desc    *prometheus.Desc
	started chan struct{}
	once    sync.Once
	err     error
	mu      sync.Mutex
}

func newRouterBlockingCollector() *routerBlockingCollector {
	return &routerBlockingCollector{
		desc:    prometheus.NewDesc("aegiscore_router_context_test", "Router context propagation test metric.", nil, nil),
		started: make(chan struct{}),
	}
}

func (c *routerBlockingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *routerBlockingCollector) Collect(ch chan<- prometheus.Metric) {
	c.CollectContext(context.Background(), ch)
}

func (c *routerBlockingCollector) CollectContext(ctx context.Context, ch chan<- prometheus.Metric) {
	c.once.Do(func() { close(c.started) })
	<-ctx.Done()
	c.mu.Lock()
	c.err = ctx.Err()
	c.mu.Unlock()
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 0)
}

func (c *routerBlockingCollector) ctxErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}
