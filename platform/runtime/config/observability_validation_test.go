package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadValidatesPprof 验证 pprof 地址和启用状态的基础校验。
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

// TestLoadValidatesPprofPort 验证 pprof listener 端口必须位于合法 TCP 端口范围内。
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

// TestLoadRejectsPprofNonLoopbackInProduction 验证生产类环境启用 pprof 时必须绑定 loopback。
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

// TestLoadAllowsPprofLoopbackInProduction 验证生产类环境允许 loopback pprof 地址。
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

// TestLoadValidatesLogAndMetrics 验证日志枚举和 metrics path 的共享校验规则。
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

// TestLoadValidatesTracing 验证 tracing sample ratio、OTLP endpoint 和生产安全约束。
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
