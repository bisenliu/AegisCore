package datastore

import (
	"database/sql"
	"fmt"

	"github.com/aegiscore/common/runtime/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenPostgres(name string, dbCfg config.PostgresDBConfig) (*sql.DB, error) {
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
