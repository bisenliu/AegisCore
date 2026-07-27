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

// NewRedisClient 创建一个 Redis Cluster 客户端，并注册单资源 Fx lifecycle hook。
func NewRedisClient(lc fx.Lifecycle, log *zap.Logger, name string, redisCfg resources.RedisConfig, options ...RedisClientOption) (redis.UniversalClient, error) {
	redisCfg.ApplyDefaults()
	client, err := openRedisClient(redisCfg, options...)
	if err != nil {
		return nil, fmt.Errorf("open redis %s: %w", name, err)
	}
	redisLog := logger.NamedComponent(log, "redis", "redis")

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, redisCfg.Timeout)
			defer cancel()
			if err := client.Ping(pingCtx).Err(); err != nil {
				return errors.Join(
					fmt.Errorf("ping redis %s: %w", name, err),
					closeRedisClient(name, client),
				)
			}
			logger.WithContext(ctx, redisLog).Info("redis connected", zap.String(logger.ResourceField, name), zap.String("mode", redisCfg.Mode), zap.Int("seed_count", len(redisCfg.Addrs)))
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

func closeRedisClient(name string, client redis.UniversalClient) error {
	if err := client.Close(); err != nil {
		return fmt.Errorf("close redis %s: %w", name, err)
	}
	return nil
}
