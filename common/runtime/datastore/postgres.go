package datastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	// 注册 OpenPostgres 使用的 pgx database/sql driver。
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aegiscore/common/runtime/resources"
)

const postgresDriver = "pgx"

// OpenPostgres 打开单个具名 PostgreSQL 连接池并应用连接池设置，但不执行启动探测。
func OpenPostgres(name string, cfg resources.PostgresConfig) (*sql.DB, error) {
	cfg.ApplyDefaults()
	db, err := sql.Open(postgresDriver, postgresDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", name, err)
	}
	applyPostgresPoolConfig(db, cfg.Pool)
	return db, nil
}

// PingPostgres 使用稳定的单资源 timeout 执行启动探测，失败时关闭连接池。
func PingPostgres(ctx context.Context, name string, db *sql.DB) error {
	pingCtx, cancel := context.WithTimeout(ctx, resources.DefaultPostgresPingTimeout())
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return errors.Join(
			fmt.Errorf("ping postgres %s: %w", name, err),
			ClosePostgres(name, db),
		)
	}
	return nil
}

// ClosePostgres 关闭连接池并保留资源名称。
func ClosePostgres(name string, db *sql.DB) error {
	if err := db.Close(); err != nil {
		return fmt.Errorf("close postgres %s: %w", name, err)
	}
	return nil
}

// PostgresDSN 根据共享资源配置构造 pgx 使用的连接字符串。
func PostgresDSN(cfg resources.PostgresConfig) string {
	cfg.ApplyDefaults()
	return postgresDSN(cfg)
}

func postgresDSN(cfg resources.PostgresConfig) string {
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
