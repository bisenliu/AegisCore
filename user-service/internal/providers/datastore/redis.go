package datastore

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
)

type redisClientFactory func(fx.Lifecycle, *zap.Logger, string, commonresources.RedisConfig, ...datastore.RedisClientOption) (redis.UniversalClient, error)

// RedisParams 包含供应单个 Redis 客户端所需的 Fx 输入。
type RedisParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Settings  serviceconfig.ResourceSettings
	Log       *zap.Logger
	Trace     *commontracing.Provider
}

// ProvideRedis 返回按调用方资源名选择配置并创建单个 Redis 客户端的构造器。
//
// 资源名由 composition root 显式传入，constructor 不写死名称，也不遍历配置 map 连接未登记资源。
func ProvideRedis(name string) func(RedisParams) (redis.UniversalClient, error) {
	return func(params RedisParams) (redis.UniversalClient, error) {
		return newRedis(name, params, datastore.NewRedisClient)
	}
}

// newRedis 根据配置的资源名称创建缓存连接客户端，并注册到系统启动检查与安全关闭生命周期中。
func newRedis(name string, params RedisParams, createRedisClient redisClientFactory) (redis.UniversalClient, error) {
	if name == "" {
		return nil, errors.New("redis resource name is required")
	}
	if params.Trace == nil {
		return nil, errors.New("redis tracing provider is required")
	}
	if createRedisClient == nil {
		return nil, errors.New("redis client factory is required")
	}
	redisCfg, ok := params.Settings.Redis[name]
	if !ok {
		return nil, fmt.Errorf("redis config %q not found", name)
	}
	client, err := createRedisClient(
		params.Lifecycle,
		params.Log,
		name,
		redisCfg,
		datastore.WithRedisTracerProvider(params.Trace.OTelTracerProvider()),
	)
	if err != nil {
		return nil, fmt.Errorf("provide redis %s: %w", name, err)
	}
	return client, nil
}
