package datastore

import (
	"database/sql"
	"fmt"

	// Register the pgx database/sql driver used by OpenPostgres.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aegiscore/common/runtime/config"
)

// OpenPostgres 打开 PostgreSQL 连接池并应用连接池设置，但不执行 ping。
func OpenPostgres(name string, dbCfg config.PostgresDBConfig) (*sql.DB, error) {
	db, err := sql.Open(dbCfg.Driver, dbCfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", name, err)
	}
	db.SetMaxOpenConns(dbCfg.MaxOpenConns)
	db.SetMaxIdleConns(dbCfg.MaxIdleConns)
	db.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(dbCfg.ConnMaxIdleTime)
	// Fx 生命周期 provider 统一执行 PingContext，使启动阶段一致报告依赖可用性。
	return db, nil
}
