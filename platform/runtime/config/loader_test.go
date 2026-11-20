package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// loadConfigFromYAML 使用完整共享 Config 默认值和校验加载一段测试 YAML。
func loadConfigFromYAML(t *testing.T, content string) *Config {
	t.Helper()
	return loadIntoFromYAML(t, content, DecodeOptions[Config]{Defaults: DefaultConfig, Validate: Config.Validate})
}

// loadConfigErrorFromYAML 加载预期失败的测试 YAML，并返回严格解码或校验错误。
func loadConfigErrorFromYAML(t *testing.T, content string) error {
	t.Helper()
	settings, mergeErr := DeepMergeYAML([]ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.NoError(t, mergeErr)
	_, err := DecodeStrict(settings, DecodeOptions[Config]{Defaults: DefaultConfig, Validate: Config.Validate})
	require.Error(t, err)
	return err
}

// loadIntoFromYAML 使用调用方提供的 DecodeOptions 加载任意扩展配置类型。
func loadIntoFromYAML[T any](t *testing.T, content string, options DecodeOptions[T]) *T {
	t.Helper()
	settings, err := DeepMergeYAML([]ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.NoError(t, err)
	cfg, err := DecodeStrict(settings, options)
	require.NoError(t, err)
	return cfg
}

// assertConfigLoadErrorContains 断言聚合配置错误中包含多个字段路径或错误片段。
func assertConfigLoadErrorContains(t *testing.T, err error, parts ...string) {
	t.Helper()
	for _, part := range parts {
		require.Contains(t, err.Error(), part)
	}
}

// configYAMLWithSection 用单个顶层 section 替换默认完整测试配置中的同名 section。
func configYAMLWithSection(section string) string {
	return configYAMLWithSections(section)
}

// configYAMLWithSections 用多个顶层 section 替换默认完整测试配置中的同名 section。
func configYAMLWithSections(sections ...string) string {
	content := explicitConfigYAML()
	for _, section := range sections {
		name := sectionName(section)
		content = replaceTopLevelSection(content, name, section)
	}
	return content
}

// sectionName 从 YAML section 文本首行解析顶层 section 名称。
func sectionName(section string) string {
	line := strings.SplitN(section, "\n", 2)[0]
	return strings.TrimSuffix(strings.TrimSpace(line), ":")
}

// replaceTopLevelSection 替换 YAML 文本中的顶层 section；不存在时追加到文末。
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

// explicitConfigYAML 返回声明所有共享 runtime 配置字段的基线测试文档。
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
    trusted_proxies:
      - 10.0.0.0/8
      - 192.0.2.10
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
