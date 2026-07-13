package providers

import (
	"errors"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/user-service/internal/resources"
)

// NamedRedisParams 包含供应用户服务 Redis 客户端所需的 Fx 输入。
type NamedRedisParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
	Trace     *commontracing.Provider
}

// NamedRedisClients 包含 user-service 使用的具名 cache Redis 客户端。
type NamedRedisClients struct {
	fx.Out

	CacheRedis *redis.Client `name:"cache_redis"`
}

// ProvideRedisClients 根据共享 datastore 配置供应具名 cache Redis 依赖。
func ProvideRedisClients(params NamedRedisParams) (NamedRedisClients, error) {
	if params.Trace == nil || params.Trace.TracerProvider() == nil {
		return NamedRedisClients{}, errors.New("redis tracing provider is required")
	}
	cacheRedis, err := datastore.NewRedisClient(
		params.Lifecycle,
		params.Config,
		params.Log,
		resources.NameCacheRedis,
		datastore.WithRedisTracerProvider(params.Trace.TracerProvider()),
	)
	if err != nil {
		return NamedRedisClients{}, err
	}
	return NamedRedisClients{CacheRedis: cacheRedis}, nil
}
