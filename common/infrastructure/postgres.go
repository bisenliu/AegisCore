package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func ProvideNamedPostgres(logicalName string, configName string) fx.Option {
	return fx.Provide(fx.Annotate(
		func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*sql.DB, error) {
			return NewPostgres(lc, cfg, log, configName)
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, logicalName)),
	))
}

func ProvideNamedRedis(logicalName string, configName string) fx.Option {
	return fx.Provide(fx.Annotate(
		func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*redis.Client, error) {
			return NewRedisClient(lc, cfg, log, configName)
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, logicalName)),
	))
}

func NewPostgres(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, name string) (*sql.DB, error) {
	dbCfg, ok := cfg.Postgres(name)
	if !ok {
		return nil, fmt.Errorf("postgres config %q not found", name)
	}
	db, err := openPostgres(name, dbCfg)
	if err != nil {
		return nil, err
	}
	registerDBLifecycle(lc, log, name, db, dbCfg)
	return db, nil
}

func openPostgres(name string, dbCfg config.PostgresDatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open(dbCfg.Driver, dbCfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", name, err)
	}
	db.SetMaxOpenConns(dbCfg.MaxOpenConns)
	db.SetMaxIdleConns(dbCfg.MaxIdleConns)
	db.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(dbCfg.ConnMaxIdleTime)
	return db, nil
}

func registerDBLifecycle(lc fx.Lifecycle, log *zap.Logger, name string, db *sql.DB, cfg config.PostgresDatabaseConfig) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
			defer cancel()
			if err := db.PingContext(pingCtx); err != nil {
				return fmt.Errorf("ping postgres %s: %w", name, err)
			}
			logger.WithContext(log, ctx).Info("postgres connected", zap.String("name", name))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := db.Close(); err != nil {
				return fmt.Errorf("close postgres %s: %w", name, err)
			}
			logger.WithContext(log, ctx).Info("postgres closed", zap.String("name", name))
			return nil
		},
	})
}
