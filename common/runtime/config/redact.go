package config

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// RedactSettings 按调用方提供的路径返回 settings 的脱敏副本。
func RedactSettings(settings map[string]any, paths []string) map[string]any {
	if settings == nil {
		return nil
	}
	copied := cloneSettings(settings).(map[string]any)
	for _, path := range paths {
		redactPath(copied, splitConfigPath(path))
	}
	return copied
}

// RenderYAML 将配置 map 渲染为 YAML，默认调用方应先执行 RedactSettings。
func RenderYAML(settings map[string]any) ([]byte, error) {
	return yaml.Marshal(settings)
}

func cloneSettings(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = cloneSettings(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			out[index] = cloneSettings(child)
		}
		return out
	default:
		return typed
	}
}

func redactPath(value any, parts []string) {
	if len(parts) == 0 {
		return
	}
	if items, ok := value.([]any); ok {
		for _, child := range items {
			redactPath(child, parts)
		}
		return
	}
	settings, ok := value.(map[string]any)
	if !ok {
		return
	}
	if parts[0] == "" {
		return
	}
	if len(parts) == 1 {
		if parts[0] == "*" {
			for key := range settings {
				settings[key] = "***"
			}
			return
		}
		if _, ok := settings[parts[0]]; ok {
			settings[parts[0]] = "***"
		}
		return
	}
	if parts[0] == "*" {
		for _, child := range settings {
			redactPath(child, parts[1:])
		}
		return
	}
	redactPath(settings[parts[0]], parts[1:])
}

func splitConfigPath(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return strings.Split(path, ".")
}
