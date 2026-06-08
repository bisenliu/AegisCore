package configfx

import "github.com/aegiscore/common/runtime/config"

// ConfigPath 是 Fx 应用传递给 config.Load 的可选文件系统路径。
type ConfigPath string

// NewConfig 为 Fx 依赖注入加载共享运行时配置。
func NewConfig(path ConfigPath) (*config.Config, error) {
	return config.Load(string(path))
}
