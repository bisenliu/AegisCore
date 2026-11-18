package datastore

import (
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/internal/resources"
)

const (
	primaryDBTag  = `name:"primary_db"`
	cacheRedisTag = `name:"cache_redis"`
)

// WiringModule 注册 user-service 显式选择的 datastore provider。
var WiringModule = fx.Module("user-service-providers-datastore",
	fx.Provide(
		// Fx 分类：资源 - 服务显式登记的 PostgreSQL 与 Redis 客户端。
		fx.Annotate(ProvidePostgres(resources.NamePrimaryDB), fx.ResultTags(primaryDBTag)),
		fx.Annotate(ProvideRedis(resources.NameCacheRedis), fx.ResultTags(cacheRedisTag)),
		// Fx 分类：资源 - 与本服务 Ent schema 相同的具名客户端。
		ProvideEntPlugins,
		fx.Annotate(ProvideEntClient(resources.NamePrimaryDB), fx.ParamTags("", "", primaryDBTag), fx.ResultTags(primaryDBTag)),
	),
)
