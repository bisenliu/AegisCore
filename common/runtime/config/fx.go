package config

// ConfigPath 是 Fx 应用传递给 Load 的可选文件系统路径。
type ConfigPath string

// NewConfig 为 Fx 依赖注入加载共享运行时配置。
func NewConfig(path ConfigPath) (*Config, error) {
	return Load(string(path))
}
