package config

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// envPrefix 定义环境变量配置覆盖使用的全局前缀。
const envPrefix = "AEGISCORE"

// Load 从指定路径或默认 configs 目录读取 YAML 配置，并应用 AEGISCORE_ 环境变量覆盖。
func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigType("yaml")
	if path != "" {
		v.SetConfigFile(path)
	} else {
		// 两个搜索路径分别支持从模块目录和服务子目录运行命令。
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath("../configs")
	}

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	for _, key := range v.AllKeys() {
		// 只绑定已发现的 key，确保具名 Redis/Postgres 实例来自已加载配置结构。
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}
