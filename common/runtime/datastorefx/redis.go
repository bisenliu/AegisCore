package datastorefx

import (
	"context"
	"fmt"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func ProvideNamedRedis(fxName string, configKey string) fx.Option {
	return fx.Provide(fx.Annotate(
		func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*redis.Client, error) {
			return NewRedisClient(lc, cfg, log, configKey)
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, fxName)),
	))
}

func NewRedisClient(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, name string) (*redis.Client, error) {
	redisCfg, ok := cfg.RedisConfig(name)
	if !ok {
		return nil, fmt.Errorf("redis config %q not found", name)
	}
	client := datastore.NewRedisClient(redisCfg)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, redisCfg.PingTimeout)
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
