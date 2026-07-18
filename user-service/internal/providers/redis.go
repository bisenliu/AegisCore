package providers

import (
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/datastore"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/resources"
)

type redisClientFactory func(fx.Lifecycle, *zap.Logger, string, commonresources.RedisConfig, ...datastore.RedisClientOption) (*redis.Client, error)

// CacheRedisParams 包含供应 user-service cache Redis 客户端所需的 Fx 输入。
type CacheRedisParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *serviceconfig.Config
	Log       *zap.Logger
	Trace     *commontracing.Provider
}

// NewCacheRedis 显式选择并供应 user-service cache Redis 客户端。
func NewCacheRedis(params CacheRedisParams) (*redis.Client, error) {
	return newCacheRedis(params, datastore.NewRedisClient)
}

func newCacheRedis(params CacheRedisParams, createRedisClient redisClientFactory) (*redis.Client, error) {
	if params.Trace == nil {
		return nil, errors.New("redis tracing provider is required")
	}
	if createRedisClient == nil {
		return nil, errors.New("redis client factory is required")
	}
	redisCfg, ok := params.Config.Resources.Redis[resources.NameCacheRedis]
	if !ok {
		return nil, fmt.Errorf("redis config %q not found", resources.NameCacheRedis)
	}
	client, err := createRedisClient(
		params.Lifecycle,
		params.Log,
		resources.NameCacheRedis,
		redisCfg,
		datastore.WithRedisTracerProvider(params.Trace.OTelTracerProvider()),
	)
	if err != nil {
		return nil, fmt.Errorf("provide redis %s: %w", resources.NameCacheRedis, err)
	}
	return client, nil
}
