package timezone

import "go.uber.org/fx"

// Module 在 Fx 构图阶段初始化进程时区。
var Module = fx.Module("aegiscore-common-timezone",
	// Fx 分类：基础运行时 - 在构图阶段完成进程级时区初始化。
	fx.Invoke(Init),
)
