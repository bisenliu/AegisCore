package bootstrap

import (
	"github.com/aegiscore/common/runtime/config"
	commoninfra "github.com/aegiscore/common/runtime/infrastructure"
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
	cacheRedis, err := commoninfra.NewRedisClient(params.Lifecycle, params.Config, params.Log, commoninfra.NameCacheRedis)
	if err != nil {
		return NamedRedisClients{}, err
	}
	return NamedRedisClients{CacheRedis: cacheRedis}, nil
}
