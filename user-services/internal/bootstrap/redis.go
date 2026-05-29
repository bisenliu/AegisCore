package bootstrap

import (
	"github.com/aegiscore/common/config"
	commoninfra "github.com/aegiscore/common/infrastructure"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const cacheRedisName = "cache_redis"

type RedisParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
}

type RedisClients struct {
	fx.Out

	CacheRedis *redis.Client `name:"cache_redis"`
}

func NewRedisClients(params RedisParams) (RedisClients, error) {
	cacheRedis, err := commoninfra.NewRedisClient(params.Lifecycle, params.Config, params.Log, cacheRedisName)
	if err != nil {
		return RedisClients{}, err
	}
	return RedisClients{CacheRedis: cacheRedis}, nil
}
