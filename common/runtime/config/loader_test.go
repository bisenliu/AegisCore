package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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

func TestLoadAppliesCoreDefaults(t *testing.T) {
	cfg := loadConfigFromYAML(t, "{}\n")

	require.Equal(t, DefaultConfig(), *cfg)
}

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

func TestLoadValidatesLifecycle(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`runtime:
  lifecycle:
    start_timeout: 0s
    stop_timeout: 0s
  gin:
    mode: sometimes
  timezone: Invalid/Timezone`))

	assertConfigLoadErrorContains(t, err,
		"runtime.lifecycle.start_timeout must be > 0",
		"runtime.lifecycle.stop_timeout must be > 0",
		"runtime.gin.mode must be one of debug, release, test",
		"runtime.timezone must be a valid IANA timezone",
	)
}

func TestLoadValidatesPprof(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: false
    sample_ratio: 0.25
    insecure: false
  pprof:
    enabled: false
    addr: invalid`))

	assertConfigLoadErrorContains(t, err,
		"observability.pprof.addr must be a host:port address",
	)
}

func TestLoadValidatesPprofPort(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: false
    sample_ratio: 0.25
    insecure: false
  pprof:
    enabled: false
    addr: 127.0.0.1:70000`))

	assertConfigLoadErrorContains(t, err,
		"observability.pprof.addr port must be between 1 and 65535",
	)
}

func TestLoadRejectsPprofNonLoopbackInProduction(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSections(`app:
  name: aegiscore-test
  environment: staging`, `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: false
    sample_ratio: 0.25
    insecure: false
  pprof:
    enabled: true
    addr: 0.0.0.0:6060`))

	assertConfigLoadErrorContains(t, err,
		"observability.pprof.addr must use a loopback address in production-like environments",
	)
}

func TestLoadAllowsPprofLoopbackInProduction(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSections(`app:
  name: aegiscore-test
  environment: production`, `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: false
    sample_ratio: 0.25
    insecure: false
  pprof:
    enabled: true
    addr: localhost:6060`))

	require.True(t, cfg.Observability.Pprof.Enabled)
	require.Equal(t, "localhost:6060", cfg.Observability.Pprof.Addr)
}

func TestLoadValidatesLifecycleStopTimeoutCoversServerShutdown(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSections(`runtime:
  lifecycle:
    start_timeout: 1s
    stop_timeout: 6s`, `server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 18080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 7s
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 8s`))

	assertConfigLoadErrorContains(t, err,
		"runtime.lifecycle.stop_timeout must be >= server.http.shutdown_timeout",
		"runtime.lifecycle.stop_timeout must be >= server.grpc.shutdown_timeout",
	)
}

func TestLoadValidatesLifecycleStopTimeoutCoversCombinedShutdownBudget(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSections(`runtime:
  lifecycle:
    start_timeout: 1s
    stop_timeout: 44s`, `server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 18080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 5s
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 4s`))

	assertConfigLoadErrorContains(t, err,
		"runtime.lifecycle.stop_timeout must be at least 45s to cover shutdown budget",
	)
}

func TestLoadAllowsLifecycleStopTimeoutAtCombinedShutdownBudget(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSections(`runtime:
  lifecycle:
    start_timeout: 1s
    stop_timeout: 45s`, `server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 18080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 5s
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 4s`))

	require.Equal(t, 45*time.Second, cfg.Runtime.Lifecycle.StopTimeout)
}

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

func TestLoadValidatesMissingCoreFields(t *testing.T) {
	err := loadConfigErrorFromYAML(t, `app:
  name: " "
  environment: " "
server:
  http:
    enabled: false
  grpc:
    enabled: false
log: {}
observability:
  metrics: {}
  tracing: {}
`)

	assertConfigLoadErrorContains(t, err,
		"app.name is required",
		"app.environment is required",
		"server must enable at least one of server.http or server.grpc",
	)
}

func TestLoadValidatesEnabledServers(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`server:
  http:
    enabled: true
    host: " "
    port: 70000
    read_timeout: 0s
    write_timeout: 0s
    idle_timeout: 0s
    shutdown_timeout: 0s
  grpc:
    enabled: true
    host: " "
    port: 0
    shutdown_timeout: 0s`))

	assertConfigLoadErrorContains(t, err,
		"server.http.host is required",
		"server.http.port must be between 1 and 65535",
		"server.http.read_timeout must be > 0",
		"server.http.write_timeout must be > 0",
		"server.http.idle_timeout must be > 0",
		"server.http.shutdown_timeout must be > 0",
		"server.grpc.host is required",
		"server.grpc.port must be between 1 and 65535",
		"server.grpc.shutdown_timeout must be > 0",
	)
}

func TestLoadAllowsDisabledHTTPWithoutPlaceholdersWhenGRPCEnabled(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`server:
  http:
    enabled: false
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 5s`))

	require.False(t, cfg.Server.HTTP.Enabled)
	require.True(t, cfg.Server.GRPC.Enabled)
}

func TestLoadValidatesLogAndMetrics(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSections(`log:
  level: trace
  format: text`, `observability:
  metrics:
    enabled: false
    path: metrics
  tracing:
    enabled: false
    sample_ratio: -0.1`))

	assertConfigLoadErrorContains(t, err,
		"log.level must be one of debug, info, warn, error",
		"log.format must be one of json, console",
		"observability.metrics.path must start with /",
		"observability.tracing.sample_ratio must be between 0 and 1",
	)
}

func TestLoadValidatesTracing(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSections(`app:
  name: aegiscore-test
  environment: prod`, `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.1
    insecure: true`))

	assertConfigLoadErrorContains(t, err,
		"observability.tracing.sample_ratio must be between 0 and 1",
		"observability.tracing.otlp_endpoint is required when tracing is enabled",
		"observability.tracing.insecure must not be true when tracing is enabled in production-like environments",
	)
}

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
	require.Equal(t, 12*time.Second, cfg.Server.GRPC.ShutdownTimeout)
	require.False(t, cfg.Observability.Metrics.Enabled)
	require.Equal(t, 0.5, cfg.Observability.Tracing.SampleRatio)
	require.False(t, cfg.Observability.Pprof.Enabled)
	require.Equal(t, "127.0.0.1:26060", cfg.Observability.Pprof.Addr)
}

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

func TestDecodeStrictWrapsValidationError(t *testing.T) {
	_, err := DecodeStrict(map[string]any{}, DecodeOptions[Config]{
		Defaults: DefaultConfig,
		Validate: func(Config) error {
			return errors.New("rejected")
		},
	})
	require.EqualError(t, err, "validate runtime config: rejected")
}

func TestDecodeStrictRequiresDefaults(t *testing.T) {
	_, err := DecodeStrict[Config](map[string]any{}, DecodeOptions[Config]{})
	require.EqualError(t, err, "decode runtime config: defaults function is required")
}

func loadConfigFromYAML(t *testing.T, content string) *Config {
	t.Helper()
	return loadIntoFromYAML(t, content, DecodeOptions[Config]{Defaults: DefaultConfig, Validate: Config.Validate})
}

func loadConfigErrorFromYAML(t *testing.T, content string) error {
	t.Helper()
	settings, mergeErr := DeepMergeYAML([]ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.NoError(t, mergeErr)
	_, err := DecodeStrict(settings, DecodeOptions[Config]{Defaults: DefaultConfig, Validate: Config.Validate})
	require.Error(t, err)
	return err
}

func loadIntoFromYAML[T any](t *testing.T, content string, options DecodeOptions[T]) *T {
	t.Helper()
	settings, err := DeepMergeYAML([]ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.NoError(t, err)
	cfg, err := DecodeStrict(settings, options)
	require.NoError(t, err)
	return cfg
}

func assertConfigLoadErrorContains(t *testing.T, err error, parts ...string) {
	t.Helper()
	for _, part := range parts {
		require.Contains(t, err.Error(), part)
	}
}

func configYAMLWithSection(section string) string {
	return configYAMLWithSections(section)
}

func configYAMLWithSections(sections ...string) string {
	content := explicitConfigYAML()
	for _, section := range sections {
		name := sectionName(section)
		content = replaceTopLevelSection(content, name, section)
	}
	return content
}

func sectionName(section string) string {
	line := strings.SplitN(section, "\n", 2)[0]
	return strings.TrimSuffix(strings.TrimSpace(line), ":")
}

func replaceTopLevelSection(content string, name string, replacement string) string {
	lines := strings.Split(content, "\n")
	start := -1
	end := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(line, name+":") {
			start = i
			continue
		}
		if start >= 0 && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			end = i
			break
		}
	}
	if start < 0 {
		return content + "\n" + replacement + "\n"
	}
	repl := strings.Split(replacement, "\n")
	updated := append([]string{}, lines[:start]...)
	updated = append(updated, repl...)
	updated = append(updated, lines[end:]...)
	return strings.Join(updated, "\n")
}

func explicitConfigYAML() string {
	return `app:
  name: aegiscore-test
  environment: local
runtime:
  lifecycle:
    start_timeout: 11s
    stop_timeout: 50s
  gin:
    mode: test
  timezone: UTC
server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 18080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 5s
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 8s
log:
  level: info
  format: json
observability:
  metrics:
    enabled: true
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 0.25
    otlp_endpoint: collector:4317
    insecure: false
  pprof:
    enabled: true
    addr: 127.0.0.1:16060
`
}
