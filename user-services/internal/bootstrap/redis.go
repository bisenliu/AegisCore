package bootstrap

import (
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastorefx"
	"github.com/aegiscore/common/runtime/resources"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type NamedRedisParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
}

type NamedRedisClients struct {
	fx.Out

	CacheRedis *redis.Client `name:"cache_redis"`
}

func ProvideRedisClients(params NamedRedisParams) (NamedRedisClients, error) {
	cacheRedis, err := datastorefx.NewRedisClient(params.Lifecycle, params.Config, params.Log, resources.NameCacheRedis)
	if err != nil {
		return NamedRedisClients{}, err
	}
	return NamedRedisClients{CacheRedis: cacheRedis}, nil
}
