package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestLoadValidatesLifecycle 验证 lifecycle start/stop timeout 必须为正数。
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

// TestLoadValidatesLifecycleStopTimeoutCoversServerShutdown 验证总停止预算不能短于 server shutdown timeout。
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

// TestLoadValidatesLifecycleStopTimeoutCoversCombinedShutdownBudget 验证总停止预算覆盖组合关闭余量。
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
    trusted_proxies:
      - 10.0.0.0/8
      - 192.0.2.10
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 4s`))

	assertConfigLoadErrorContains(t, err,
		"runtime.lifecycle.stop_timeout must be at least 45s to cover shutdown budget",
	)
}

// TestLoadAllowsLifecycleStopTimeoutAtCombinedShutdownBudget 验证达到组合关闭预算下限时校验通过。
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
    trusted_proxies:
      - 10.0.0.0/8
      - 192.0.2.10
  grpc:
    enabled: true
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 4s`))

	require.Equal(t, 45*time.Second, cfg.Runtime.Lifecycle.StopTimeout)
}

// TestLoadValidatesMissingCoreFields 验证 app、runtime、server 和 log 必填字段缺失会失败。
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

// TestLoadValidatesEnabledServers 验证至少启用 HTTP 或 gRPC 中的一个 server。
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

// TestLoadValidatesHTTPTrustedProxies 验证 trusted proxy 只接受 IP 或 CIDR。
func TestLoadValidatesHTTPTrustedProxies(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 18080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 5s
    trusted_proxies:
      - 10.0.0.0/33
      - " "
  grpc:
    enabled: false`))

	assertConfigLoadErrorContains(t, err,
		"server.http.trusted_proxies[0] must be an IP address or CIDR",
		"server.http.trusted_proxies[1] must be an IP address or CIDR",
	)
}

// TestLoadAllowsDisabledHTTPWithoutPlaceholdersWhenGRPCEnabled 验证禁用 HTTP 时不要求 HTTP 占位字段。
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
