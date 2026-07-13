package config

import commonconfig "github.com/aegiscore/common/runtime/config"

// ConfigPath 是 Fx 应用传递给 Load 的可选文件系统路径。
type ConfigPath string

// NewConfig 为 Fx 依赖注入加载 user-service 配置。
func NewConfig(path ConfigPath) (*Config, error) {
	return commonconfig.LoadInto(string(path), Config.Validate)
}

// NewRuntimeConfig 提供共享 runtime 配置，供 common provider 消费。
func NewRuntimeConfig(cfg *Config) *commonconfig.Config {
	runtime := cfg.RuntimeConfig()
	return &runtime
}
