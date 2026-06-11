package timezone

import (
	"github.com/aegiscore/common/runtime/config"
	"go.uber.org/fx"
)

// Params 包含初始化进程时区所需的 Fx 依赖。
type Params struct {
	fx.In

	Config *config.Config
}

// Init 根据 Fx 提供的配置初始化进程时区。
func Init(params Params) error {
	return InitConfig(params.Config)
}

// Module 在 Fx 启动阶段初始化进程时区。
var Module = fx.Module("aegiscore-common-timezone",
	fx.Invoke(Init),
)
