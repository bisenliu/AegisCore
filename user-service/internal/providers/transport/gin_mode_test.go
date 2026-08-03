package transport

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

func TestConfigureGinModeUsesRuntimeConfig(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })
	cfg := ginTestConfig()
	cfg.Runtime.Gin.Mode = gin.DebugMode

	configured, err := ConfigureGinMode(cfg)

	require.NoError(t, err)
	require.Equal(t, GinModeConfigured{}, configured)
	require.Equal(t, gin.DebugMode, gin.Mode())
}

func TestNewGinEngineDoesNotOverrideConfiguredGinMode(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })
	gin.SetMode(gin.TestMode)
	cfg := ginTestConfig()
	provider := newGinTestTracingProvider(t, cfg)

	_, err := NewGinEngine(GinParams{Config: cfg, Trace: provider, HTTP: ginTestHTTPSettings()})

	require.NoError(t, err)
	require.Equal(t, gin.TestMode, gin.Mode())
}

func TestProviderGraphConfiguresGinModeBeforeEngine(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.App = config.AppConfig{Name: "configured-user-service", Environment: "test"}
	cfg.Runtime.Gin.Mode = gin.ReleaseMode
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: false, Path: "/metrics", IncludeRuntime: true}
	cfg.Observability.Tracing = config.TracingConfig{Enabled: false, SampleRatio: 1}

	var engine *gin.Engine
	app := fxtest.New(t,
		fx.Supply(&cfg, zap.NewNop(), ginTestHTTPSettings()),
		fx.Provide(ConfigureGinMode, commonmetrics.NewMetricsProvider, commontracing.NewTracingProvider, NewGinEngine),
		fx.Populate(&engine),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, engine)
	require.Equal(t, gin.ReleaseMode, gin.Mode())
}
