package providers

import "go.uber.org/fx"

// Module 将用户服务级基础设施 provider 接入 Fx。
var Module = fx.Module("user-service-providers",
	fx.Provide(
		ProvidePostgresPools,
		ProvideRedisClients,
		NewJWTService,
		ProvideEntClients,
		NewGinEngine,
	),
	fx.Invoke(RegisterRoutes),
)
