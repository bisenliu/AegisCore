package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestNewGinEngineRejectsOversizedBodyBeforeRouteHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true, Path: "/metrics"}
	traceProvider := newGinTestTracingProvider(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{
		Config:  cfg,
		Trace:   traceProvider,
		Metrics: metricsProvider,
		HTTP:    serviceconfig.HTTPSettings{RequestBodyMaxBytes: 8},
	})
	require.NoError(t, err)
	handled := false
	engine.POST("/body", func(c *gin.Context) {
		handled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/body", strings.NewReader("123456789")))

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.False(t, handled)
	var envelope contractresponse.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, contracterrors.CodeRequestBodyTooLarge, envelope.Code)
	metric := findGinMetricByLabels(t, gatherGinMetricFamily(t, metricsProvider, "http_server_requests_total"), map[string]string{
		commonmetrics.LabelMethod:      http.MethodPost,
		commonmetrics.LabelRoute:       "/body",
		commonmetrics.LabelStatusClass: "4xx",
	})
	require.Equal(t, float64(1), metric.GetCounter().GetValue())
}

func TestNewGinEngineRejectsInvalidRequestBodyLimit(t *testing.T) {
	cfg := ginTestConfig()
	traceProvider := newGinTestTracingProvider(t, cfg)

	engine, err := NewGinEngine(GinParams{Config: cfg, Trace: traceProvider, HTTP: serviceconfig.HTTPSettings{}})

	require.Nil(t, engine)
	require.EqualError(t, err, "configure request body limit: request body max bytes must be > 0")
}
