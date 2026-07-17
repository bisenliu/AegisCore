package datastore

import (
	"context"
	"database/sql"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
)

// NewPostgres 创建一个 PostgreSQL 连接池并注册单资源 Fx lifecycle hook。
func NewPostgres(lc fx.Lifecycle, log *zap.Logger, name string, cfg resources.PostgresConfig) (*sql.DB, error) {
	db, err := OpenPostgres(name, cfg)
	if err != nil {
		return nil, err
	}
	registerPostgresLifecycle(lc, log, name, db)
	return db, nil
}

func registerPostgresLifecycle(lc fx.Lifecycle, log *zap.Logger, name string, db *sql.DB) {
	postgresLog := logger.NamedComponent(log, "postgres", "postgres")
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := PingPostgres(ctx, name, db); err != nil {
				return err
			}
			logger.WithContext(ctx, postgresLog).Info("postgres connected", zap.String(logger.ResourceField, name))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := ClosePostgres(name, db); err != nil {
				return err
			}
			logger.WithContext(ctx, postgresLog).Info("postgres closed", zap.String(logger.ResourceField, name))
			return nil
		},
	})
}
