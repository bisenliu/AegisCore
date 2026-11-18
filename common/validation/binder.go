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

// BindValues 将 URL 查询参数（Query 参数）或表单键值对按标签名称自动解析并填充到目标结构体中。
// 优先级由传入的标签名称顺序决定；对于可选的过滤条件结构体，只有在请求中真正传入了相关参数时才会创建对象，避免生成多余的空对象。
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

// bindStruct 读取表单或请求中的各项内容，并逐一填入到数据对象的对应字段中；遇到数据类型不匹配或必填项缺失时返回错误。
func bindStruct(value reflect.Value, values url.Values, tags []string) error {
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		structField := typ.Field(i)
		if structField.PkgPath != "" && !structField.Anonymous {
			continue
		}
		if structField.Anonymous && field.Kind() == reflect.Pointer && field.Type().Elem().Kind() == reflect.Struct {
			// 只有当请求参数中确实包含了该嵌套对象的字段时才为其创建内存，避免在用户未传递过滤条件时生成无意义的空结构体。
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

// embeddedStructHasValues 检查子数据对象中是否实际填写了数据，避免把没有任何内容的空子对象当作有效输入。
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

// bindingName 从 json、form 等标签取出绑定用的字段名。
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

// setField 将单个请求输入值转换并写入到对应的字段中；如果格式无法转换则返回错误。
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

// setScalar 将文本形式的输入内容转换成目标基础类型（如数字、布尔值或时间）并保存。
func setScalar(field reflect.Value, raw string) error {
	if field.CanAddr() {
		if unmarshaler, ok := field.Addr().Interface().(encoding.TextUnmarshaler); ok {
			// 若目标字段自定义了文本解析规则（如枚举类型实现了 TextUnmarshaler 接口），则优先调用其自定义解析方法将字符串转换为枚举对象。
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
