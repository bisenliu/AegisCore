package infrastructure

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func NewRedisClient(lc fx.Lifecycle, cfg *config.Config, log *slog.Logger, name string) (*redis.Client, error) {
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
			log.InfoContext(ctx, "redis connected", slog.String("name", name), slog.String("addr", redisCfg.Addr))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := client.Close(); err != nil {
				return fmt.Errorf("close redis %s: %w", name, err)
			}
			log.InfoContext(ctx, "redis closed", slog.String("name", name))
			return nil
		},
	})

	return client, nil
}
