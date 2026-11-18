package config

import (
	"reflect"
	"sort"
	"strings"
)

const mapstructureTag = "mapstructure"

// unknownConfigPaths 将 mapstructure 报告的未消费键展开为完整叶子路径。
func unknownConfigPaths(settings map[string]any, unused []string) string {
	paths := make([]string, 0, len(unused))
	for _, unusedPath := range unused {
		value := any(settings)
		for _, name := range strings.Split(unusedPath, ".") {
			children, ok := value.(map[string]any)
			if !ok {
				break
			}
			value = children[name]
		}
		paths = append(paths, unknownLeafPaths(value, unusedPath)...)
	}
	sort.Strings(paths)
	return strings.Join(paths, ", ")
}

// configFieldName 读取配置项在文本文件中的对应名称，并识别该项是否需要忽略或并入上一级配置。
func configFieldName(field reflect.StructField) (name string, squash bool, skip bool) {
	tag := field.Tag.Get(mapstructureTag)
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return "", false, true
	}
	for _, option := range parts[1:] {
		if option == "squash" {
			squash = true
		}
	}
	if squash {
		return "", true, false
	}
	if parts[0] != "" {
		return parts[0], false, false
	}
	return strings.ToLower(field.Name), false, false
}

// unknownLeafPaths 找出配置文件中写了但系统根本不需要的无效多余配置项，生成完整的层级路径提示用户修改（例如 a.b.c）。
func unknownLeafPaths(value any, prefix string) []string {
	settings, ok := value.(map[string]any)
	if !ok || len(settings) == 0 {
		return []string{prefix}
	}
	var paths []string
	for key, child := range settings {
		paths = append(paths, unknownLeafPaths(child, joinConfigPath(prefix, key))...)
	}
	return paths
}

// joinConfigPath 将父级名称与当前配置项名称用圆点连接（例如 server.port），拼接出完整的配置层级路径。
func joinConfigPath(prefix string, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
