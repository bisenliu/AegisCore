package validation

import "go.uber.org/fx"

// Module 为 Fx 应用提供默认 validation.Validator。
var Module = fx.Module("validation", fx.Provide(NewDefault))
