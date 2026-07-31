package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// entSQLLogPlugin 在显式启用时把 Ent driver 包装为 SQL 结构化日志 driver。
type entSQLLogPlugin struct {
	log           *zap.Logger
	db            string
	debugEnabled  bool
	slowThreshold time.Duration
	now           func() time.Time
}

// WrapEntDriver 返回带 SQL log 的 driver；默认值只在插件启用路径内生效。
func (p *entSQLLogPlugin) WrapEntDriver(driver dialect.Driver) (dialect.Driver, error) {
	log := p.log
	if log == nil {
		log = zap.NewNop()
	}
	db := p.db
	if db == "" {
		db = primaryDatabaseResource
	}
	slowThreshold := p.slowThreshold
	if slowThreshold <= 0 {
		slowThreshold = serviceconfig.DefaultEntSlowQueryThreshold
	}
	now := p.now
	if now == nil {
		now = time.Now
	}
	return newEntSQLLogDriver(driver, log, db, p.debugEnabled, slowThreshold, now), nil
}

// entSQLLogDriver 记录 Ent SQL 执行、查询和事务边界的低基数字段。
type entSQLLogDriver struct {
	dialect.Driver
	log           *zap.Logger
	db            string
	debugEnabled  bool
	slowThreshold time.Duration
	now           func() time.Time
}

// entSQLLogTx 为事务内 Exec/Query/Commit/Rollback 复用同一 SQL log driver。
type entSQLLogTx struct {
	dialect.Tx
	driver *entSQLLogDriver
	ctx    context.Context
}

// newEntSQLLogDriver 构造 SQL log driver，并补齐测试可替换的时钟和慢查询阈值。
func newEntSQLLogDriver(driver dialect.Driver, log *zap.Logger, db string, debugEnabled bool, slowThreshold time.Duration, now func() time.Time) *entSQLLogDriver {
	if log == nil {
		log = zap.NewNop()
	}
	if db == "" {
		db = primaryDatabaseResource
	}
	if slowThreshold <= 0 {
		slowThreshold = serviceconfig.DefaultEntSlowQueryThreshold
	}
	if now == nil {
		now = time.Now
	}
	return &entSQLLogDriver{
		Driver:        driver,
		log:           log,
		db:            db,
		debugEnabled:  debugEnabled,
		slowThreshold: slowThreshold,
		now:           now,
	}
}

func (d *entSQLLogDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.observe(ctx, sqlOperation(query, "exec"), query, func() error {
		return d.Driver.Exec(ctx, query, args, v)
	})
}

func (d *entSQLLogDriver) Query(ctx context.Context, query string, args, v any) error {
	return d.observe(ctx, sqlOperation(query, "query"), query, func() error {
		return d.Driver.Query(ctx, query, args, v)
	})
}

func (d *entSQLLogDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	start := d.now()
	tx, err := d.Driver.Tx(ctx)
	d.logOperation(ctx, "tx.begin", "", d.now().Sub(start), err)
	if err != nil {
		return nil, err
	}
	return &entSQLLogTx{Tx: tx, driver: d, ctx: ctx}, nil
}

func (d *entSQLLogDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	start := d.now()
	driver, ok := d.Driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	})
	if !ok {
		err := fmt.Errorf("ent driver does not support transaction options")
		d.logOperation(ctx, "tx.begin", "", d.now().Sub(start), err)
		return nil, err
	}
	tx, err := driver.BeginTx(ctx, opts)
	d.logOperation(ctx, "tx.begin", "", d.now().Sub(start), err)
	if err != nil {
		return nil, err
	}
	return &entSQLLogTx{Tx: tx, driver: d, ctx: ctx}, nil
}

func (d *entSQLLogDriver) observe(ctx context.Context, operation string, statement string, call func() error) error {
	start := d.now()
	err := call()
	d.logOperation(ctx, operation, statement, d.now().Sub(start), err)
	return err
}

func (d *entSQLLogDriver) logOperation(ctx context.Context, operation string, statement string, duration time.Duration, err error) {
	fields := []zap.Field{
		zap.String("db", d.db),
		zap.String("operation", operation),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}
	if statement != "" {
		fields = append(fields, zap.String("statement", statement))
	}
	log := logger.WithContext(ctx, d.log)
	switch {
	case err != nil:
		log.Error("ent sql failed", append(fields, zap.Error(err))...)
	case d.slowThreshold > 0 && duration >= d.slowThreshold:
		log.Warn("ent sql slow", fields...)
	case d.debugEnabled:
		log.Debug("ent sql completed", fields...)
	}
}

func (tx *entSQLLogTx) Exec(ctx context.Context, query string, args, v any) error {
	return tx.driver.observe(ctx, sqlOperation(query, "tx.exec"), query, func() error {
		return tx.Tx.Exec(ctx, query, args, v)
	})
}

func (tx *entSQLLogTx) Query(ctx context.Context, query string, args, v any) error {
	return tx.driver.observe(ctx, sqlOperation(query, "tx.query"), query, func() error {
		return tx.Tx.Query(ctx, query, args, v)
	})
}

func (tx *entSQLLogTx) Commit() error {
	return tx.driver.observe(tx.ctx, "tx.commit", "", tx.Tx.Commit)
}

func (tx *entSQLLogTx) Rollback() error {
	return tx.driver.observe(tx.ctx, "tx.rollback", "", tx.Tx.Rollback)
}

// sqlOperation 从 SQL 语句中提取稳定操作名，空语句返回调用方提供的 fallback。
func sqlOperation(statement string, fallback string) string {
	fields := strings.Fields(strings.TrimSpace(statement))
	if len(fields) == 0 {
		return fallback
	}
	return strings.ToLower(strings.Trim(fields[0], `"()`))
}
