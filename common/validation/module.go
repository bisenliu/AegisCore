package validation

import "go.uber.org/fx"

var Module = fx.Module("validation", fx.Provide(NewDefault))
