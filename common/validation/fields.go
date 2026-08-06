package validation

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// fieldName 按显式 label 和各绑定来源 tag 的优先级选择对外字段名。
func fieldName(fld reflect.StructField) string {
	if label := fld.Tag.Get(TagLabel); label != "" {
		return label
	}
	for _, tag := range [...]string{TagJSON, TagForm, TagHeader, TagURI, TagQuery} {
		name := strings.SplitN(fld.Tag.Get(tag), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return fld.Name
}

// displayName 将 decoder 报告的点分隔字段路径解析为面向调用方的末级字段名称。
func displayName(dst any, path string) string {
	if path == "" {
		return "参数"
	}
	typ := reflect.TypeOf(dst)
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil || typ.Kind() != reflect.Struct {
		return path
	}
	return displayNameFromType(typ, strings.Split(path, "."))
}

func displayNameFromType(typ reflect.Type, parts []string) string {
	if len(parts) == 0 {
		return typ.Name()
	}
	part := parts[0]
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonName := strings.SplitN(field.Tag.Get(TagJSON), ",", 2)[0]
		if jsonName != part && field.Name != part {
			continue
		}
		if len(parts) == 1 {
			return displayNameFromField(field)
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct {
			return displayNameFromType(fieldType, parts[1:])
		}
		return displayNameFromField(field)
	}
	return part
}

func displayNameFromField(field reflect.StructField) string {
	if label := field.Tag.Get(TagLabel); label != "" {
		return label
	}
	if jsonName := strings.SplitN(field.Tag.Get(TagJSON), ",", 2)[0]; jsonName != "" && jsonName != "-" {
		return jsonName
	}
	return fieldName(field)
}

func validationFieldNames(dst any, fieldErr validator.FieldError) (string, string) {
	// 优先从 StructNamespace 重建请求字段路径，失败时退回 validator 提供的 Go 字段名。
	field, label, ok := validationFieldNamesFromPath(dst, fieldErr.StructNamespace())
	if ok {
		return field, label
	}
	name := fieldErr.Field()
	return name, name
}

func validationFieldNamesFromPath(dst any, namespace string) (string, string, bool) {
	// validator namespace 的首段是根类型名，不属于对外请求字段路径。
	parts := strings.Split(namespace, ".")
	if len(parts) < 2 {
		return "", "", false
	}
	typ := reflect.TypeOf(dst)
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil || typ.Kind() != reflect.Struct {
		return "", "", false
	}

	fieldParts := make([]string, 0, len(parts)-1)
	label := ""
	for _, part := range parts[1:] {
		// slice/array 元素带有 [n] 后缀；反射查字段前先剥离索引，响应路径仍保持字段级粒度。
		part = strings.SplitN(part, "[", 2)[0]
		structField, ok := typ.FieldByName(part)
		if !ok {
			return "", "", false
		}
		name := requestFieldName(structField)
		if name == "" {
			return "", "", false
		}
		fieldParts = append(fieldParts, name)
		label = displayNameFromField(structField)

		typ = structField.Type
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			break
		}
	}
	if len(fieldParts) == 0 || label == "" {
		return "", "", false
	}
	return strings.Join(fieldParts, "."), label, true
}

func requestFieldName(field reflect.StructField) string {
	// tag 优先级与 Binder 支持的请求来源保持一致，避免错误响应暴露 Go 字段名。
	for _, tag := range [...]string{TagJSON, TagForm, TagHeader, TagURI, TagQuery} {
		name := strings.SplitN(field.Tag.Get(tag), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return field.Name
}
