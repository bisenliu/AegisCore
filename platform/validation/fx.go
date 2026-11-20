package validation

import "go.uber.org/fx"

// Module 为 Fx 应用提供默认 validation.Validator。
var Module = fx.Module("validation",
	// Fx 分类：横切能力 - 请求输入校验与错误归一化。
	fx.Provide(NewDefault),
)
