package datastorefx

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func ProvideNamedPostgres(fxName string, configKey string) fx.Option {
	return fx.Provide(fx.Annotate(
		func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*sql.DB, error) {
			return NewPostgres(lc, cfg, log, configKey)
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, fxName)),
	))
}

func NewPostgres(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, name string) (*sql.DB, error) {
	dbCfg, ok := cfg.PostgresDatabase(name)
	if !ok {
		return nil, fmt.Errorf("postgres config %q not found", name)
	}
	db, err := datastore.OpenPostgres(name, dbCfg)
	if err != nil {
		return nil, err
	}
	registerDBLifecycle(lc, log, name, db, dbCfg)
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
