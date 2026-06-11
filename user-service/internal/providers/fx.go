package providers

import "go.uber.org/fx"

// Module wires user-service level infrastructure providers into Fx.
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
