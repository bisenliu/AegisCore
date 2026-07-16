package providers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/user-service/ent"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// NamedEntClientParams 包含由具名 SQL 连接池支撑 Ent client 所需的 Fx 输入。
type NamedEntClientParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *serviceconfig.Config
	Log       *zap.Logger
	Metrics   *commonmetrics.Provider
	Tracing   *commontracing.Provider
	UserDB    *sql.DB `name:"user_db"`
}

// NamedEntClients 包含绑定用户服务数据库的 Ent client Fx 输出。
type NamedEntClients struct {
	fx.Out

	UserClient *ent.Client `name:"user_db"`
}

type nonClosingEntDriver struct {
	dialect.Driver
}

const (
	userDatabaseResource         = "user_db"
	defaultEntSlowQueryThreshold = 500 * time.Millisecond
)

type entObservabilityDriver struct {
	dialect.Driver
	log           *zap.Logger
	db            string
	debugEnabled  bool
	slowThreshold time.Duration
	now           func() time.Time
}

type entObservabilityTx struct {
	dialect.Tx
	driver *entObservabilityDriver
	ctx    context.Context
}

// ProvideEntClients 将具名 SQL 连接池包装为 Ent client，并注册 Ent client 关闭 hook。
func ProvideEntClients(params NamedEntClientParams) (NamedEntClients, error) {
	userClient, err := newEntClient(params.UserDB, params.Config, logger.SQL(params.Log), params.Metrics, params.Tracing)
	if err != nil {
		return NamedEntClients{}, err
	}

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, params.Log).Info("closing ent clients")
			return closeEntClient("user_db", userClient.Close)
		},
	})

	return NamedEntClients{UserClient: userClient}, nil
}

func newEntClient(db *sql.DB, cfg *serviceconfig.Config, sqlLog *zap.Logger, metricsProvider *commonmetrics.Provider, tracingProvider *commontracing.Provider) (*ent.Client, error) {
	client := ent.NewClient(ent.Driver(newEntDriver(db, cfg, sqlLog)))
	if err := installEntObservability(client, metricsProvider, tracingProvider); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func newEntDriver(db *sql.DB, cfg *serviceconfig.Config, sqlLog *zap.Logger) dialect.Driver {
	// SQL 连接池由 datastore 生命周期 hook 持有，Ent client 不应独立关闭它们。
	driver := nonClosingEntDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}
	return newEntObservabilityDriver(driver, sqlLog, userDatabaseResource, entSQLDebugEnabled(cfg))
}

func entSQLDebugEnabled(cfg *serviceconfig.Config) bool {
	return cfg != nil && cfg.Ent.SQLDebug
}

func newEntObservabilityDriver(driver dialect.Driver, log *zap.Logger, db string, debugEnabled bool) *entObservabilityDriver {
	if log == nil {
		log = zap.NewNop()
	}
	return &entObservabilityDriver{
		Driver:        driver,
		log:           log,
		db:            db,
		debugEnabled:  debugEnabled,
		slowThreshold: defaultEntSlowQueryThreshold,
		now:           time.Now,
	}
}

func (d *entObservabilityDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.observe(ctx, sqlOperation(query, "exec"), query, func() error {
		return d.Driver.Exec(ctx, query, args, v)
	})
}

func (d *entObservabilityDriver) Query(ctx context.Context, query string, args, v any) error {
	return d.observe(ctx, sqlOperation(query, "query"), query, func() error {
		return d.Driver.Query(ctx, query, args, v)
	})
}

func (d *entObservabilityDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	start := d.now()
	tx, err := d.Driver.Tx(ctx)
	d.logOperation(ctx, "tx.begin", "", d.now().Sub(start), err)
	if err != nil {
		return nil, err
	}
	return &entObservabilityTx{Tx: tx, driver: d, ctx: ctx}, nil
}

func (d *entObservabilityDriver) observe(ctx context.Context, operation string, statement string, call func() error) error {
	start := d.now()
	err := call()
	d.logOperation(ctx, operation, statement, d.now().Sub(start), err)
	return err
}

func (d *entObservabilityDriver) logOperation(ctx context.Context, operation string, statement string, duration time.Duration, err error) {
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

func (tx *entObservabilityTx) Exec(ctx context.Context, query string, args, v any) error {
	return tx.driver.observe(ctx, sqlOperation(query, "tx.exec"), query, func() error {
		return tx.Tx.Exec(ctx, query, args, v)
	})
}

func (tx *entObservabilityTx) Query(ctx context.Context, query string, args, v any) error {
	return tx.driver.observe(ctx, sqlOperation(query, "tx.query"), query, func() error {
		return tx.Tx.Query(ctx, query, args, v)
	})
}

func (tx *entObservabilityTx) Commit() error {
	return tx.driver.observe(tx.ctx, "tx.commit", "", tx.Tx.Commit)
}

func (tx *entObservabilityTx) Rollback() error {
	return tx.driver.observe(tx.ctx, "tx.rollback", "", tx.Tx.Rollback)
}

func sqlOperation(statement string, fallback string) string {
	fields := strings.Fields(strings.TrimSpace(statement))
	if len(fields) == 0 {
		return fallback
	}
	return strings.ToLower(strings.Trim(fields[0], `"()`))
}

func (d nonClosingEntDriver) Close() error {
	// Close 有意保持为空操作，只屏蔽 Ent client 对底层连接池的关闭，不改变 Query/Exec 行为。
	return nil
}

func closeEntClient(name string, closeClient func() error) error {
	if err := closeClient(); err != nil {
		return fmt.Errorf("close %s ent client: %w", name, err)
	}
	return nil
}
