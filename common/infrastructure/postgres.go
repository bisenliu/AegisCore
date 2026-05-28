package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/aegiscore/common/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/fx"
)

func NewPostgres(lc fx.Lifecycle, cfg *config.Config, log *slog.Logger, name string) (*sql.DB, error) {
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

func registerDBLifecycle(lc fx.Lifecycle, log *slog.Logger, name string, db *sql.DB, cfg config.PostgresDatabaseConfig) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
			defer cancel()
			if err := db.PingContext(pingCtx); err != nil {
				return fmt.Errorf("ping postgres %s: %w", name, err)
			}
			log.InfoContext(ctx, "postgres connected", slog.String("name", name))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := db.Close(); err != nil {
				return fmt.Errorf("close postgres %s: %w", name, err)
			}
			log.InfoContext(ctx, "postgres closed", slog.String("name", name))
			return nil
		},
	})
}
