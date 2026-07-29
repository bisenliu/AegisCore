package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

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
	span := endedGinSpan(t, provider, recorder)
	require.Equal(t, tracepb.Status_STATUS_CODE_ERROR, span.GetStatus().GetCode())
	require.True(t, spanHasEvent(span, "exception"), "events=%#v", span.GetEvents())
}
