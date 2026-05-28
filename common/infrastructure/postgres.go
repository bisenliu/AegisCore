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

const (
	UserDBName   = "user_db"
	CommonDBName = "common_db"
)

type PostgresPools struct {
	fx.Out

	UserDB   *sql.DB `name:"user_db"`
	CommonDB *sql.DB `name:"common_db"`
}

func NewPostgresPools(lc fx.Lifecycle, cfg *config.Config, log *slog.Logger) (PostgresPools, error) {
	userDB, err := openPostgres(cfg, UserDBName)
	if err != nil {
		return PostgresPools{}, err
	}
	commonDB, err := openPostgres(cfg, CommonDBName)
	if err != nil {
		_ = userDB.Close()
		return PostgresPools{}, err
	}

	userDBCfg, _ := cfg.Postgres(UserDBName)
	commonDBCfg, _ := cfg.Postgres(CommonDBName)
	registerDBLifecycle(lc, log, UserDBName, userDB, userDBCfg)
	registerDBLifecycle(lc, log, CommonDBName, commonDB, commonDBCfg)

	return PostgresPools{UserDB: userDB, CommonDB: commonDB}, nil
}

func openPostgres(cfg *config.Config, name string) (*sql.DB, error) {
	dbCfg, ok := cfg.Postgres(name)
	if !ok {
		return nil, fmt.Errorf("postgres config %q not found", name)
	}
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
