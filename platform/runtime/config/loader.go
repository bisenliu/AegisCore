package config

import (
	"fmt"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// DecodeOptions 显式定义严格解码的默认值、归一化和校验步骤；Defaults 必填，其他步骤可选。
type DecodeOptions[T any] struct {
	Defaults  func() T
	Normalize func(*T)
	Validate  func(T) error
}

// DecodeStrict 按 defaults、raw settings 覆盖、未知键检查、normalize、validate 的顺序解码配置。
func DecodeStrict[T any](settings map[string]any, options DecodeOptions[T]) (*T, error) {
	if options.Defaults == nil {
		return nil, fmt.Errorf("decode runtime config: defaults function is required")
	}
	target := options.Defaults()

	v := viper.New()
	setViperDefaultsFromStruct(v, "", reflect.ValueOf(target))
	if err := v.MergeConfigMap(settings); err != nil {
		return nil, fmt.Errorf("decode runtime config: %w", err)
	}
	metadata := new(mapstructure.Metadata)
	if err := v.Unmarshal(&target, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)), func(config *mapstructure.DecoderConfig) {
		config.Metadata = metadata
	}); err != nil {
		return nil, fmt.Errorf("decode runtime config: %w", err)
	}
	if paths := unknownConfigPaths(settings, metadata.Unused); paths != "" {
		return nil, fmt.Errorf("decode runtime config: unknown configuration keys: %s", paths)
	}
	if options.Normalize != nil {
		options.Normalize(&target)
	}
	if options.Validate != nil {
		if err := options.Validate(target); err != nil {
			return nil, fmt.Errorf("validate runtime config: %w", err)
		}
	}
	return &target, nil
}

// setViperDefaultsFromStruct 将显式 defaults 按 mapstructure tag 注册给 Viper，使嵌套 map 也支持 raw 局部覆盖。
func setViperDefaultsFromStruct(v *viper.Viper, prefix string, value reflect.Value) {
	value = dereferenceValue(value)
	if !value.IsValid() {
		return
	}
	if value.Kind() != reflect.Struct {
		if prefix != "" {
			v.SetDefault(prefix, value.Interface())
		}
		return
	}

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
		fieldValue := value.Field(index)
		if squash {
			setViperDefaultsFromStruct(v, prefix, fieldValue)
			continue
		}
		setViperDefaultsFromStruct(v, joinConfigPath(prefix, name), fieldValue)
	}
}

// dereferenceValue 解开指针默认值；nil 指针没有可注册的默认配置叶子。
func dereferenceValue(value reflect.Value) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}
