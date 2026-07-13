package validation

import (
	"encoding"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// BindValues 按给定 struct tag 顺序扫描，将 URL 风格 values 绑定到结构体指针。
// tag 按调用方传入顺序决定字段名优先级；匿名指针结构体只有出现相关输入时才分配，以保留可选过滤器语义。
func BindValues(dst any, values url.Values, tags ...string) error {
	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("validation bind target must be a non-nil pointer")
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return fmt.Errorf("validation bind target must point to a struct")
	}
	return bindStruct(value, values, tags)
}

func bindStruct(value reflect.Value, values url.Values, tags []string) error {
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		structField := typ.Field(i)
		if structField.PkgPath != "" && !structField.Anonymous {
			continue
		}
		if structField.Anonymous && field.Kind() == reflect.Pointer && field.Type().Elem().Kind() == reflect.Struct {
			// 嵌入指针结构体只有在其 tag 字段出现时才分配，保留可选过滤器语义。
			if field.IsNil() && embeddedStructHasValues(field.Type().Elem(), values, tags) {
				field.Set(reflect.New(field.Type().Elem()))
			}
			if !field.IsNil() {
				if err := bindStruct(field.Elem(), values, tags); err != nil {
					return err
				}
			}
			continue
		}
		if structField.Anonymous && field.Kind() == reflect.Struct {
			if err := bindStruct(field, values, tags); err != nil {
				return err
			}
			continue
		}
		name := bindingName(structField, tags)
		if name == "" {
			continue
		}
		rawValues, ok := values[name]
		if !ok || len(rawValues) == 0 {
			continue
		}
		if err := setField(field, rawValues); err != nil {
			return &bindFieldError{field: displayNameFromField(structField), typ: field.Type(), err: err}
		}
	}
	return nil
}

func embeddedStructHasValues(typ reflect.Type, values url.Values, tags []string) bool {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := bindingName(field, tags)
		if name != "" {
			if rawValues, ok := values[name]; ok && len(rawValues) > 0 {
				return true
			}
		}
		fieldType := field.Type
		if field.Anonymous {
			for fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct && embeddedStructHasValues(fieldType, values, tags) {
				return true
			}
		}
	}
	return false
}

func bindingName(field reflect.StructField, tags []string) string {
	for _, tag := range tags {
		name := strings.SplitN(field.Tag.Get(tag), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return ""
}

func setField(field reflect.Value, rawValues []string) error {
	if !field.CanSet() {
		return nil
	}
	if field.Kind() == reflect.Pointer {
		if rawValues[0] == "" {
			// 空字符串保持指针字段为 nil，避免将省略的可选值混淆为零值。
			return nil
		}
		field.Set(reflect.New(field.Type().Elem()))
		return setField(field.Elem(), rawValues)
	}
	if field.Kind() == reflect.Slice {
		// 重复 query/form 值映射为切片；标量字段只使用首个值以保持绑定可预测。
		slice := reflect.MakeSlice(field.Type(), 0, len(rawValues))
		for _, raw := range rawValues {
			elem := reflect.New(field.Type().Elem()).Elem()
			if err := setScalar(elem, raw); err != nil {
				return err
			}
			slice = reflect.Append(slice, elem)
		}
		field.Set(slice)
		return nil
	}
	return setScalar(field, rawValues[0])
}

func setScalar(field reflect.Value, raw string) error {
	if field.CanAddr() {
		if unmarshaler, ok := field.Addr().Interface().(encoding.TextUnmarshaler); ok {
			// 自定义文本反序列化是枚举等领域类型的扩展点。
			return unmarshaler.UnmarshalText([]byte(raw))
		}
	}
	if field.Type() == reflect.TypeOf(time.Duration(0)) {
		// duration 请求输入使用类似 "5s" 的 Go 语法，而不是原始整数纳秒。
		value, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		field.SetInt(int64(value))
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(value)
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(value)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}
