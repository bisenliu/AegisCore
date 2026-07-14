package config

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const mapstructureTag = "mapstructure"

// validateKnownConfigKeys 根据目标配置结构拒绝所有未声明的 YAML 路径。
func validateKnownConfigKeys(target any, settings map[string]any) error {
	paths := unknownConfigPaths(reflect.TypeOf(target), settings, "")
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	return fmt.Errorf("unknown configuration keys: %s", strings.Join(paths, ", "))
}

func unknownConfigPaths(targetType reflect.Type, value any, prefix string) []string {
	targetType = indirectType(targetType)
	settings, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	switch targetType.Kind() {
	case reflect.Struct:
		fields := configFields(targetType)
		var paths []string
		for key, child := range settings {
			path := joinConfigPath(prefix, key)
			fieldType, exists := fields[key]
			if !exists {
				paths = append(paths, unknownLeafPaths(child, path)...)
				continue
			}
			paths = append(paths, unknownConfigPaths(fieldType, child, path)...)
		}
		return paths
	case reflect.Map:
		elementType := indirectType(targetType.Elem())
		if elementType.Kind() != reflect.Struct && elementType.Kind() != reflect.Map {
			return nil
		}
		var paths []string
		for key, child := range settings {
			paths = append(paths, unknownConfigPaths(elementType, child, joinConfigPath(prefix, key))...)
		}
		return paths
	default:
		return nil
	}
}

func configFields(targetType reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for index := 0; index < targetType.NumField(); index++ {
		field := targetType.Field(index)
		name, squash, skip := configFieldName(field)
		if skip {
			continue
		}
		if squash {
			for nestedName, nestedType := range configFields(indirectType(field.Type)) {
				fields[nestedName] = nestedType
			}
			continue
		}
		fields[name] = field.Type
	}
	return fields
}

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

func indirectType(targetType reflect.Type) reflect.Type {
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	return targetType
}

func joinConfigPath(prefix string, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
