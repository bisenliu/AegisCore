package providers

import (
	"go.uber.org/fx"

	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

// Module 将用户服务级基础设施 provider 接入 Fx。
var Module = fx.Module("user-service-providers",
	fx.Provide(
		commontracing.NewFxProvider,
		ProvidePostgresPools,
		ProvideRedisClients,
		NewJWTService,
		ProvideEntClients,
		ProvideHealthChecks,
		NewGinEngine,
	),
	fx.Invoke(RegisterRoutes),
)
