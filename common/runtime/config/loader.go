package config

import (
	"fmt"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type defaultsApplier interface {
	ApplyDefaults()
}

type defaultsProvider interface {
	ConfigDefaults() map[string]any
}

// DecodeStrict 将已合成的配置 map 严格解码到调用方目标结构。
func DecodeStrict[T any](settings map[string]any, validate func(T) error) (*T, error) {
	var target T
	v := viper.New()
	setCoreDefaults(v, DefaultConfig())
	var serviceDefaults T
	if provider, ok := any(&serviceDefaults).(defaultsProvider); ok {
		for path, value := range provider.ConfigDefaults() {
			v.SetDefault(path, value)
		}
	}
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
	if defaults, ok := any(&target).(defaultsApplier); ok {
		// 服务扩展配置可在校验前补齐自身默认值，并将结果保留在返回对象中。
		defaults.ApplyDefaults()
	}
	if validate != nil {
		if err := validate(target); err != nil {
			return nil, fmt.Errorf("validate runtime config: %w", err)
		}
	}
	return &target, nil
}

func setCoreDefaults(v *viper.Viper, defaults Config) {
	setViperDefaultsFromStruct(v, "", reflect.ValueOf(defaults))
}

// setViperDefaultsFromStruct 按 mapstructure tag 递归展开默认配置，避免在 loader 中维护另一份字段路径映射。
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
