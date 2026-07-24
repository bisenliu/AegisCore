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

type viperDefaultsApplier interface {
	ApplyViperDefaults(*viper.Viper)
}

// LoadInto 从单个 YAML 文件读取调用方指定的完整配置结构。
func LoadInto[T any](path string, validate func(T) error) (*T, error) {
	if path == "" {
		return nil, fmt.Errorf("read config: config file path is required")
	}
	v := viper.New()
	setCoreDefaults(v, DefaultConfig())
	var serviceDefaults T
	if defaults, ok := any(&serviceDefaults).(viperDefaultsApplier); ok {
		defaults.ApplyViperDefaults(v)
	}

	v.SetConfigType("yaml")
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg T
	if err := validateKnownConfigKeys(cfg, v.AllSettings()); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if defaults, ok := any(&cfg).(defaultsApplier); ok {
		// 服务扩展配置可在校验前补齐自身默认值，并将结果保留在返回对象中。
		defaults.ApplyDefaults()
	}
	if validate != nil {
		if err := validate(cfg); err != nil {
			return nil, fmt.Errorf("validate config: %w", err)
		}
	}

	return &cfg, nil
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
