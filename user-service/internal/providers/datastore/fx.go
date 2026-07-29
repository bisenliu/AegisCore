package datastore

import "go.uber.org/fx"

// WiringModule 注册 user-service 拥有的 datastore provider。
var WiringModule = fx.Module("user-service-providers-datastore",
	fx.Provide(
		// Fx 分类：资源 - 服务拥有的 PostgreSQL 与 Redis 客户端。
		fx.Annotate(NewPrimaryDB, fx.ResultTags(`name:"primary_db"`)),
		fx.Annotate(NewCacheRedis, fx.ResultTags(`name:"cache_redis"`)),
		// Fx 分类：资源 - 服务拥有的 Ent 客户端。
		ProvideEntClients,
	),
)
