package config

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// RedactSettings 按调用方提供的路径返回 settings 的脱敏副本。
//
// common 不内置任何服务私有敏感字段策略；调用方必须显式提供自身配置 schema 中
// 需要脱敏的点分路径。路径段支持使用 "*" 匹配当前 map 的全部 key，也支持在遍历
// 过程中遇到 []any 时继续将同一剩余路径应用到每个元素。
func RedactSettings(settings map[string]any, paths []string) map[string]any {
	if settings == nil {
		return nil
	}
	// 先复制再脱敏，保证 render/debug 调用不会反向污染 EffectiveSettings 或 digest 输入。
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

// cloneSettings 只深拷贝 YAML settings 里会出现的 map/slice 容器。
// 标量值保持原引用或原值；配置脱敏只会替换 map 中的叶子值，不会修改这些标量本身。
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

// redactPath 沿点分路径递归替换叶子值。
// 未命中路径、空路径段或非 map/slice 中间节点都是 no-op，便于调用方安全传入统一路径列表。
func redactPath(value any, parts []string) {
	if len(parts) == 0 {
		return
	}
	if items, ok := value.([]any); ok {
		// slice 本身不消费路径段；每个元素继续匹配同一剩余路径，覆盖列表型配置结构。
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
			// 叶子通配表示当前 map 的全部值都应被脱敏。
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
		// 中间通配只匹配 map 的下一层 key，后续路径继续决定最终叶子。
		for _, child := range settings {
			redactPath(child, parts[1:])
		}
		return
	}
	redactPath(settings[parts[0]], parts[1:])
}

// splitConfigPath 将调用方路径拆为路径段；空白路径视为无效 no-op。
func splitConfigPath(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return strings.Split(path, ".")
}
