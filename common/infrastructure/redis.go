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

func NewRedisClient(lc fx.Lifecycle, cfg *config.Config, log *slog.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Username:     cfg.Redis.Username,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := client.Ping(pingCtx).Err(); err != nil {
				return fmt.Errorf("ping redis: %w", err)
			}
			log.InfoContext(ctx, "redis connected", slog.String("addr", cfg.Redis.Addr))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := client.Close(); err != nil {
				return fmt.Errorf("close redis: %w", err)
			}
			log.InfoContext(ctx, "redis closed")
			return nil
		},
	})

	return client, nil
}
