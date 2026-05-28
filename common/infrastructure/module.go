package infrastructure

import "go.uber.org/fx"

var Module = fx.Module("aegiscore-common-infrastructure",
	fx.Provide(
		NewConfig,
		NewLogger,
	),
)
