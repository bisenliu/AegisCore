package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConfigContainsOnlyCoreGroups 验证 common Config 只包含跨服务核心配置组。
func TestConfigContainsOnlyCoreGroups(t *testing.T) {
	typeOfConfig := reflect.TypeOf(Config{})
	require.Equal(t, 5, typeOfConfig.NumField())

	for index, expected := range []struct {
		name string
		tag  string
	}{
		{name: "App", tag: "app"},
		{name: "Runtime", tag: "runtime"},
		{name: "Server", tag: "server"},
		{name: "Log", tag: "log"},
		{name: "Observability", tag: "observability"},
	} {
		field := typeOfConfig.Field(index)
		require.Equal(t, expected.name, field.Name)
		require.Equal(t, expected.tag, field.Tag.Get("mapstructure"))
	}
}

// TestDefaultConfigSupportsLocalHTTPServer 验证默认配置可直接启动本地 HTTP server。
func TestDefaultConfigSupportsLocalHTTPServer(t *testing.T) {
	cfg := DefaultConfig()

	require.NoError(t, cfg.Validate())
	require.Equal(t, DefaultAppName, cfg.App.Name)
	require.Equal(t, DefaultAppEnvironment, cfg.App.Environment)
	require.Positive(t, cfg.Runtime.Lifecycle.StartTimeout)
	require.Positive(t, cfg.Runtime.Lifecycle.StopTimeout)
	require.Equal(t, DefaultGinMode, cfg.Runtime.Gin.Mode)
	require.Equal(t, DefaultTimezone, cfg.Runtime.Timezone)
	require.True(t, cfg.Server.HTTP.Enabled)
	require.Equal(t, DefaultHTTPHost, cfg.Server.HTTP.Host)
	require.Equal(t, DefaultHTTPPort, cfg.Server.HTTP.Port)
	require.Positive(t, cfg.Server.HTTP.ReadTimeout)
	require.Positive(t, cfg.Server.HTTP.WriteTimeout)
	require.Positive(t, cfg.Server.HTTP.IdleTimeout)
	require.Positive(t, cfg.Server.HTTP.ShutdownTimeout)
	require.Empty(t, cfg.Server.HTTP.TrustedProxies)
	require.GreaterOrEqual(t, cfg.Runtime.Lifecycle.StopTimeout, cfg.Server.HTTP.ShutdownTimeout)
	require.GreaterOrEqual(t, cfg.Runtime.Lifecycle.StopTimeout, cfg.Server.GRPC.ShutdownTimeout)
	require.GreaterOrEqual(t, cfg.Runtime.Lifecycle.StopTimeout, cfg.minimumLifecycleStopBudget())
	require.False(t, cfg.Server.GRPC.Enabled)
	require.Equal(t, "info", cfg.Log.Level)
	require.Equal(t, "json", cfg.Log.Format)
	require.Equal(t, DefaultMetricsPath, cfg.Observability.Metrics.Path)
	require.False(t, cfg.Observability.Tracing.Enabled)
	require.False(t, cfg.Observability.Pprof.Enabled)
	require.Equal(t, DefaultPprofAddr, cfg.Observability.Pprof.Addr)
}

// TestLoadAppliesCoreDefaults 验证严格解码会应用共享 runtime 默认值。
func TestLoadAppliesCoreDefaults(t *testing.T) {
	cfg := loadConfigFromYAML(t, "{}\n")

	require.Equal(t, DefaultConfig(), *cfg)
}

// TestLoadExplicitConfig 验证完整显式配置能被正确解析并保留字段值。
func TestLoadExplicitConfig(t *testing.T) {
	cfg := loadConfigFromYAML(t, explicitConfigYAML())

	require.Equal(t, "aegiscore-test", cfg.App.Name)
	require.Equal(t, "local", cfg.App.Environment)
	require.Equal(t, 11*time.Second, cfg.Runtime.Lifecycle.StartTimeout)
	require.Equal(t, 50*time.Second, cfg.Runtime.Lifecycle.StopTimeout)
	require.Equal(t, "test", cfg.Runtime.Gin.Mode)
	require.Equal(t, "UTC", cfg.Runtime.Timezone)
	require.True(t, cfg.Server.HTTP.Enabled)
	require.Equal(t, "127.0.0.1", cfg.Server.HTTP.Host)
	require.Equal(t, 18080, cfg.Server.HTTP.Port)
	require.Equal(t, 10*time.Second, cfg.Server.HTTP.ReadTimeout)
	require.Equal(t, 20*time.Second, cfg.Server.HTTP.WriteTimeout)
	require.Equal(t, 30*time.Second, cfg.Server.HTTP.IdleTimeout)
	require.Equal(t, 5*time.Second, cfg.Server.HTTP.ShutdownTimeout)
	require.Equal(t, []string{"10.0.0.0/8", "192.0.2.10"}, cfg.Server.HTTP.TrustedProxies)
	require.True(t, cfg.Server.GRPC.Enabled)
	require.Equal(t, "127.0.0.1", cfg.Server.GRPC.Host)
	require.Equal(t, 19090, cfg.Server.GRPC.Port)
	require.Equal(t, 8*time.Second, cfg.Server.GRPC.ShutdownTimeout)
	require.Equal(t, "info", cfg.Log.Level)
	require.Equal(t, "json", cfg.Log.Format)
	require.True(t, cfg.Observability.Metrics.Enabled)
	require.Equal(t, "/metrics", cfg.Observability.Metrics.Path)
	require.True(t, cfg.Observability.Tracing.Enabled)
	require.Equal(t, 0.25, cfg.Observability.Tracing.SampleRatio)
	require.Equal(t, "collector:4317", cfg.Observability.Tracing.OTLPEndpoint)
	require.True(t, cfg.Observability.Pprof.Enabled)
	require.Equal(t, "127.0.0.1:16060", cfg.Observability.Pprof.Addr)
}
