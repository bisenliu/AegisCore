package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func NewRedisClient(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, name string) (*redis.Client, error) {
	redisCfg, ok := cfg.RedisConfig(name)
	if !ok {
		return nil, fmt.Errorf("redis config %q not found", name)
	}
	client := redis.NewClient(&redis.Options{
		Addr:         redisCfg.Addr,
		Username:     redisCfg.Username,
		Password:     redisCfg.Password,
		DB:           redisCfg.DB,
		DialTimeout:  redisCfg.DialTimeout,
		ReadTimeout:  redisCfg.ReadTimeout,
		WriteTimeout: redisCfg.WriteTimeout,
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := client.Ping(pingCtx).Err(); err != nil {
				return fmt.Errorf("ping redis %s: %w", name, err)
			}
			logger.WithContext(log, ctx).Info("redis connected", zap.String("name", name), zap.String("addr", redisCfg.Addr))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := client.Close(); err != nil {
				return fmt.Errorf("close redis %s: %w", name, err)
			}
			logger.WithContext(log, ctx).Info("redis closed", zap.String("name", name))
			return nil
		},
	})

	return client, nil
}
