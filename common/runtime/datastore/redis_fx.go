package datastore

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
)

// ProvideNamedRedis 通过 Fx 名称到配置 key 的映射提供一个具名 Redis 客户端。
func ProvideNamedRedis(fxName string, configKey string) fx.Option {
	// Fx 分类：资源 - 通用具名 Redis provider 工厂。
	return fx.Provide(fx.Annotate(
		func(lc fx.Lifecycle, configs resources.RedisConfigs, log *zap.Logger) (*redis.Client, error) {
			return NewRedisClient(lc, configs, log, configKey)
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, fxName)),
	))
}

// NewRedisClient 创建一个具名 Redis 客户端，并在 Fx 启动阶段验证可用性。
func NewRedisClient(lc fx.Lifecycle, configs resources.RedisConfigs, log *zap.Logger, name string, options ...RedisClientOption) (*redis.Client, error) {
	redisCfg, ok := configs[name]
	if !ok {
		return nil, fmt.Errorf("redis config %q not found", name)
	}
	redisCfg.ApplyDefaults()
	client := OpenRedisClient(redisCfg, options...)
	redisLog := logger.NamedComponent(log, "redis", "redis")

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Redis 不可用时启动快速失败，避免依赖服务在缓存异常状态下运行。
			pingCtx, cancel := context.WithTimeout(ctx, redisCfg.Timeout)
			defer cancel()
			if err := client.Ping(pingCtx).Err(); err != nil {
				return errors.Join(
					fmt.Errorf("ping redis %s: %w", name, err),
					closeRedisClient(name, client),
				)
			}
			logger.WithContext(ctx, redisLog).Info("redis connected", zap.String(logger.ResourceField, name), zap.String("addr", redisCfg.Addr))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := closeRedisClient(name, client); err != nil {
				return err
			}
			logger.WithContext(ctx, redisLog).Info("redis closed", zap.String(logger.ResourceField, name))
			return nil
		},
	})

	return client, nil
}

func closeRedisClient(name string, client *redis.Client) error {
	if err := client.Close(); err != nil {
		return fmt.Errorf("close redis %s: %w", name, err)
	}
	return nil
}
