package infrastructure

import "github.com/aegiscore/common/runtime/config"

type ConfigPath string

func NewConfig(path ConfigPath) (*config.Config, error) {
	return config.Load(string(path))
}
