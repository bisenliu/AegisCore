package config

import (
	"fmt"
	"reflect"
	"time"
)

var durationType = reflect.TypeOf(time.Duration(0))

// EncodeSettings 按 mapstructure tag 将已解码配置编码为可渲染的配置 map。
func EncodeSettings(value any) (map[string]any, error) {
	encoded, err := encodeSettingValue(reflect.ValueOf(value))
	if err != nil {
		return nil, err
	}
	settings, ok := encoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("encode runtime config: root value must be a struct")
	}
	return settings, nil
}

func encodeSettingValue(value reflect.Value) (any, error) {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil, nil
	}
	if value.Type() == durationType {
		return value.Interface().(time.Duration).String(), nil
	}

	switch value.Kind() {
	case reflect.Struct:
		return encodeSettingStruct(value)
	case reflect.Map:
		return encodeSettingMap(value)
	case reflect.Slice, reflect.Array:
		items := make([]any, value.Len())
		for index := 0; index < value.Len(); index++ {
			item, err := encodeSettingValue(value.Index(index))
			if err != nil {
				return nil, err
			}
			items[index] = item
		}
		return items, nil
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return value.Interface(), nil
	default:
		return nil, fmt.Errorf("encode runtime config: unsupported value type %s", value.Type())
	}
}

func encodeSettingStruct(value reflect.Value) (map[string]any, error) {
	settings := make(map[string]any)
	valueType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := valueType.Field(index)
		if field.PkgPath != "" {
			continue
		}
		name, squash, skip := configFieldName(field)
		if skip {
			continue
		}
		encoded, err := encodeSettingValue(value.Field(index))
		if err != nil {
			return nil, err
		}
		if !squash {
			settings[name] = encoded
			continue
		}
		nested, ok := encoded.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("encode runtime config: squashed field %s must be a struct", field.Name)
		}
		for nestedName, nestedValue := range nested {
			if _, exists := settings[nestedName]; exists {
				return nil, fmt.Errorf("encode runtime config: duplicate field %s", nestedName)
			}
			settings[nestedName] = nestedValue
		}
	}
	return settings, nil
}

func encodeSettingMap(value reflect.Value) (map[string]any, error) {
	if value.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("encode runtime config: map key type must be string, got %s", value.Type().Key())
	}
	settings := make(map[string]any, value.Len())
	iterator := value.MapRange()
	for iterator.Next() {
		encoded, err := encodeSettingValue(iterator.Value())
		if err != nil {
			return nil, err
		}
		settings[iterator.Key().String()] = encoded
	}
	return settings, nil
}
