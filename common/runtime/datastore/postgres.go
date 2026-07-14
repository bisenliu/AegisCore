package datastore

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"

	// 注册 OpenPostgres 使用的 pgx database/sql driver。
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aegiscore/common/runtime/resources"
)

const postgresDriver = "pgx"

// OpenPostgres 打开 PostgreSQL 连接池并应用连接池设置，但不执行 ping。
func OpenPostgres(name string, cfg resources.PostgresConfig) (*sql.DB, error) {
	cfg.ApplyDefaults()
	db, err := sql.Open(postgresDriver, PostgresDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", name, err)
	}
	applyPostgresPoolConfig(db, cfg.Pool)
	// Fx 生命周期 provider 统一执行 PingContext，使启动阶段一致报告依赖可用性。
	return db, nil
}

// PostgresDSN 根据共享资源配置构造 pgx 使用的连接字符串。
func PostgresDSN(cfg resources.PostgresConfig) string {
	cfg.ApplyDefaults()
	databasePath := "/" + cfg.DBName
	u := url.URL{
		Scheme:  "postgres",
		User:    url.UserPassword(cfg.Username, cfg.Password),
		Host:    net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:    databasePath,
		RawPath: "/" + url.PathEscape(cfg.DBName),
	}
	query := u.Query()
	query.Set("sslmode", cfg.SSLMode)
	u.RawQuery = query.Encode()
	return u.String()
}

func applyPostgresPoolConfig(db *sql.DB, cfg resources.PostgresPoolConfig) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}
