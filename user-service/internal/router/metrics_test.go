package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

func TestRegisterMetricsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("disabled does not register route", func(t *testing.T) {
		engine := gin.New()
		provider := newRouterTestMetricsProvider(t, false, "/metrics")
		err := registerMetricsRoute(engine, MetricsRouteParams{Config: metricsRouteConfig(false, "/metrics"), Provider: provider})
		if err != nil {
			t.Fatalf("registerMetricsRoute: %v", err)
		}
		recorder := executeMetricsRequest(engine, "/metrics")
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
	})

	t.Run("enabled exposes prometheus text", func(t *testing.T) {
		engine := gin.New()
		provider := newRouterTestMetricsProvider(t, true, "/metrics")
		registerRouterTestCounter(t, provider)
		err := registerMetricsRoute(engine, MetricsRouteParams{Config: metricsRouteConfig(true, "/metrics"), Provider: provider})
		if err != nil {
			t.Fatalf("registerMetricsRoute: %v", err)
		}
		recorder := executeMetricsRequest(engine, "/metrics")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
			t.Fatalf("content type = %q, want prometheus text", contentType)
		}
		if body := recorder.Body.String(); !strings.Contains(body, "aegiscore_router_test_total") {
			t.Fatalf("body = %q, want test metric family", body)
		}
	})

	t.Run("custom path does not also expose default path", func(t *testing.T) {
		engine := gin.New()
		provider := newRouterTestMetricsProvider(t, true, "/internal/metrics")
		registerRouterTestCounter(t, provider)
		err := registerMetricsRoute(engine, MetricsRouteParams{Config: metricsRouteConfig(true, "/internal/metrics"), Provider: provider})
		if err != nil {
			t.Fatalf("registerMetricsRoute: %v", err)
		}
		if recorder := executeMetricsRequest(engine, "/metrics"); recorder.Code != http.StatusNotFound {
			t.Fatalf("default status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
		if recorder := executeMetricsRequest(engine, "/internal/metrics"); recorder.Code != http.StatusOK {
			t.Fatalf("custom status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
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
			if !errors.Is(err, ErrInvalidMetricsPath) {
				t.Fatalf("error = %v, want ErrInvalidMetricsPath", err)
			}
		})
	}
}

func TestIsLowNoiseRuntimePath(t *testing.T) {
	cfg := metricsRouteConfig(true, "/metrics")
	if !IsLowNoiseRuntimePath("/livez", cfg) || !IsLowNoiseRuntimePath("/metrics", cfg) {
		t.Fatal("runtime path check = false, want health and metrics paths to be low noise")
	}
	if IsLowNoiseRuntimePath("/api/v1/users", cfg) {
		t.Fatal("business route marked low noise")
	}
	if IsLowNoiseRuntimePath("/metrics", metricsRouteConfig(false, "/metrics")) {
		t.Fatal("disabled metrics path marked low noise")
	}
}

func newRouterTestMetricsProvider(t *testing.T, enabled bool, metricsPath string) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      metricsRouteConfig(enabled, metricsPath),
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return provider
}

func registerRouterTestCounter(t *testing.T, provider *commonmetrics.Provider) {
	t.Helper()
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_router_test_total",
		Help: "Router metrics endpoint test counter.",
	})
	counter.Inc()
	if err := provider.Register(counter); err != nil {
		t.Fatalf("Register counter: %v", err)
	}
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
