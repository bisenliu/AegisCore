package config

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDecodeStrictServiceExtension 验证服务扩展配置可通过显式 DecodeOptions 接入共享解码。
func TestDecodeStrictServiceExtension(t *testing.T) {
	type extended struct {
		Config `mapstructure:",squash"`
		Auth   struct {
			JWT struct {
				Secret string `mapstructure:"secret"`
			} `mapstructure:"jwt"`
		} `mapstructure:"auth"`
	}

	cfg := loadIntoFromYAML[extended](t, explicitConfigYAML()+`auth:
  jwt:
    secret: test-secret
`, DecodeOptions[extended]{
		Defaults: func() extended {
			return extended{Config: DefaultConfig()}
		},
		Validate: func(cfg extended) error {
			return cfg.Validate()
		},
	})
	require.Equal(t, "test-secret", cfg.Auth.JWT.Secret)
	require.Equal(t, "aegiscore-test", cfg.App.Name)
}

// TestDecodeStrictNestedServiceExtension 验证嵌套服务扩展字段也参与严格解码和默认值处理。
func TestDecodeStrictNestedServiceExtension(t *testing.T) {
	type redisResource struct {
		Addr string `mapstructure:"addr"`
	}
	type extended struct {
		Config    `mapstructure:",squash"`
		Resources struct {
			Redis map[string]redisResource `mapstructure:"redis"`
		} `mapstructure:"resources"`
		Feature struct {
			Mode string `mapstructure:"mode"`
		} `mapstructure:"feature"`
	}

	cfg := loadIntoFromYAML[extended](t, `resources:
  redis:
    cache:
      addr: 127.0.0.1:6379
feature:
  mode: strict
`, DecodeOptions[extended]{
		Defaults: func() extended {
			return extended{Config: DefaultConfig()}
		},
		Validate: func(cfg extended) error {
			return cfg.Validate()
		},
	})

	require.Equal(t, "127.0.0.1:6379", cfg.Resources.Redis["cache"].Addr)
	require.Equal(t, "strict", cfg.Feature.Mode)
	require.Equal(t, DefaultHTTPPort, cfg.Server.HTTP.Port)
}

// TestDecodeStrictLoadsCompleteConfig 验证完整共享配置能被严格解码为 Config。
func TestDecodeStrictLoadsCompleteConfig(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSections(`runtime:
  lifecycle:
    start_timeout: 13s
    stop_timeout: 52s
  gin:
    mode: debug
  timezone: Asia/Shanghai`, `server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 28080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 5s
    trusted_proxies:
      - 10.0.0.0/8
      - 192.0.2.10
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 12s`, `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 0.5
    otlp_endpoint: collector:4317
    insecure: false
  pprof:
    enabled: false
    addr: 127.0.0.1:26060`))
	require.Equal(t, 13*time.Second, cfg.Runtime.Lifecycle.StartTimeout)
	require.Equal(t, 52*time.Second, cfg.Runtime.Lifecycle.StopTimeout)
	require.Equal(t, "debug", cfg.Runtime.Gin.Mode)
	require.Equal(t, "Asia/Shanghai", cfg.Runtime.Timezone)
	require.Equal(t, 28080, cfg.Server.HTTP.Port)
	require.Equal(t, []string{"10.0.0.0/8", "192.0.2.10"}, cfg.Server.HTTP.TrustedProxies)
	require.Equal(t, 12*time.Second, cfg.Server.GRPC.ShutdownTimeout)
	require.False(t, cfg.Observability.Metrics.Enabled)
	require.Equal(t, 0.5, cfg.Observability.Tracing.SampleRatio)
	require.False(t, cfg.Observability.Pprof.Enabled)
	require.Equal(t, "127.0.0.1:26060", cfg.Observability.Pprof.Addr)
}

// TestLoadRejectsUnknownLegacyKeysWithFullPaths 验证未知旧字段会以完整路径报告。
func TestLoadRejectsUnknownLegacyKeysWithFullPaths(t *testing.T) {
	tests := []struct {
		name     string
		section  string
		expected string
	}{
		{name: "system", section: "system:\n  timezone: UTC", expected: "system.timezone"},
		{name: "top level http", section: "http:\n  host: 127.0.0.1", expected: "http.host"},
		{name: "postgres", section: "postgres:\n  primary_db:\n    host: 127.0.0.1", expected: "postgres.primary_db.host"},
		{name: "redis", section: "redis:\n  cache:\n    addr: 127.0.0.1:6379", expected: "redis.cache.addr"},
		{name: "local cache", section: "local_cache:\n  auth:\n    ttl: 1s", expected: "local_cache.auth.ttl"},
		{name: "log directory", section: "log:\n  level: info\n  format: json\n  directory: ./logs", expected: "log.directory"},
		{name: "log filename", section: "log:\n  level: info\n  format: json\n  filename: app.log", expected: "log.filename"},
		{name: "log console", section: "log:\n  level: info\n  format: json\n  console: true", expected: "log.console"},
		{name: "http pprof", section: `http:
  pprof:
    enabled: true`, expected: "http.pprof.enabled"},
		{name: "http trusted proxies", section: `http:
  trusted_proxies:
    - 127.0.0.1`, expected: "http.trusted_proxies"},
		{name: "tracing exporter", section: `observability:
  metrics:
    enabled: true
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 0.25
    exporter: otlp
    otlp_endpoint: collector:4317
    insecure: false`, expected: "observability.tracing.exporter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadConfigErrorFromYAML(t, configYAMLWithSection(tt.section))
			assertConfigLoadErrorContains(t, err, "unknown configuration keys", tt.expected)
		})
	}
}

// TestLoadRejectsUnknownServiceExtensionKey 验证服务扩展中的未知字段同样被严格拒绝。
func TestLoadRejectsUnknownServiceExtensionKey(t *testing.T) {
	type extended struct {
		Config `mapstructure:",squash"`
		Auth   struct {
			Secret string `mapstructure:"secret"`
		} `mapstructure:"auth"`
	}

	settings, mergeErr := DeepMergeYAML([]ConfigDocument{{DataID: "test.yaml", Content: []byte("auth:\n  secret: test\n  legacy: rejected\n")}})
	require.NoError(t, mergeErr)
	normalized := false
	validated := false

	_, err := DecodeStrict(settings, DecodeOptions[extended]{
		Defaults: func() extended {
			return extended{Config: DefaultConfig()}
		},
		Normalize: func(*extended) {
			normalized = true
		},
		Validate: func(cfg extended) error {
			validated = true
			return cfg.Validate()
		},
	})
	require.Error(t, err)
	assertConfigLoadErrorContains(t, err, "unknown configuration keys", "auth.legacy")
	require.False(t, normalized)
	require.False(t, validated)
}

// TestDecodeStrictRunsExplicitOptionsInOrder 验证 defaults、normalize 和 validate 按显式顺序执行。
func TestDecodeStrictRunsExplicitOptionsInOrder(t *testing.T) {
	type pipelineConfig struct {
		Enabled      bool   `mapstructure:"enabled"`
		Value        string `mapstructure:"value"`
		DefaultOnly  string `mapstructure:"default_only"`
		NormalizedAt string `mapstructure:"normalized_at"`
	}

	var calls []string
	cfg, err := DecodeStrict(map[string]any{
		"enabled": false,
		"value":   "raw",
	}, DecodeOptions[pipelineConfig]{
		Defaults: func() pipelineConfig {
			calls = append(calls, "defaults")
			return pipelineConfig{Enabled: true, Value: "default", DefaultOnly: "preserved"}
		},
		Normalize: func(cfg *pipelineConfig) {
			calls = append(calls, "normalize")
			cfg.Value += "-normalized"
			cfg.NormalizedAt = "normalize"
		},
		Validate: func(cfg pipelineConfig) error {
			calls = append(calls, "validate")
			require.False(t, cfg.Enabled)
			require.Equal(t, "raw-normalized", cfg.Value)
			require.Equal(t, "preserved", cfg.DefaultOnly)
			require.Equal(t, "normalize", cfg.NormalizedAt)
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"defaults", "normalize", "validate"}, calls)
	require.False(t, cfg.Enabled)
	require.Equal(t, "raw-normalized", cfg.Value)
	require.Equal(t, "preserved", cfg.DefaultOnly)
}

// TestDecodeStrictWrapsValidationError 验证校验错误会从严格解码流程返回并保留语义。
func TestDecodeStrictWrapsValidationError(t *testing.T) {
	_, err := DecodeStrict(map[string]any{}, DecodeOptions[Config]{
		Defaults: DefaultConfig,
		Validate: func(Config) error {
			return errors.New("rejected")
		},
	})
	require.EqualError(t, err, "validate runtime config: rejected")
}

// TestDecodeStrictRequiresDefaults 验证调用方必须显式提供默认配置函数。
func TestDecodeStrictRequiresDefaults(t *testing.T) {
	_, err := DecodeStrict[Config](map[string]any{}, DecodeOptions[Config]{})
	require.EqualError(t, err, "decode runtime config: defaults function is required")
}
