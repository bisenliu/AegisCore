package datastore

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

// NamedEntClientParams 包含由具名 SQL 连接池支撑 Ent client 所需的 Fx 输入。
type NamedEntClientParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Settings  serviceconfig.EntSettings
	Log       *zap.Logger
	Metrics   *commonmetrics.Provider
	Tracing   *commontracing.Provider
	PrimaryDB *sql.DB `name:"primary_db"`
}

// NamedEntClients 包含绑定 user-service 主数据库的 Ent client Fx 输出。
type NamedEntClients struct {
	fx.Out

	PrimaryClient *ent.Client `name:"primary_db"`
}

// nonClosingEntDriver 将 Ent client 的生命周期与由 Fx 管理的底层 SQL 连接池解耦。
type nonClosingEntDriver struct {
	dialect.Driver
}

const (
	primaryDatabaseResource = "primary_db"
)

// ProvideEntClients 将具名 SQL 连接池包装为 Ent client，并注册 Ent client 关闭 hook。
func ProvideEntClients(params NamedEntClientParams) (NamedEntClients, error) {
	plugins, err := newEntPlugins(params.Settings, params.Log, params.Metrics, params.Tracing)
	if err != nil {
		return NamedEntClients{}, err
	}
	primaryClient, err := newEntClient(params.PrimaryDB, plugins)
	if err != nil {
		return NamedEntClients{}, err
	}

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, params.Log).Info("closing ent clients")
			return closeEntClient(primaryDatabaseResource, primaryClient.Close)
		},
	})

	return NamedEntClients{PrimaryClient: primaryClient}, nil
}

func newEntClient(db *sql.DB, plugins entPluginSet) (*ent.Client, error) {
	driver := newEntDriver(db)
	var err error
	for _, plugin := range plugins.driverPlugins {
		driver, err = plugin.WrapEntDriver(driver)
		if err != nil {
			return nil, err
		}
	}

	client := ent.NewClient(ent.Driver(driver))
	for _, plugin := range plugins.clientPlugins {
		if err := plugin.InstallEntClientPlugin(client); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

func newEntDriver(db *sql.DB) dialect.Driver {
	// SQL 连接池由 datastore 生命周期 hook 持有，Ent client 不应独立关闭它们。
	return nonClosingEntDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}
}

// Close 保留 Ent driver 接口，但不关闭由 datastore provider 持有的 SQL 连接池。
func (d nonClosingEntDriver) Close() error {
	return nil
}

// BeginTx 在保留事务选项的同时透传到底层支持该扩展接口的 driver。
func (d nonClosingEntDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	driver, ok := d.Driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	})
	if !ok {
		return nil, fmt.Errorf("ent driver does not support transaction options")
	}
	return driver.BeginTx(ctx, opts)
}

func closeEntClient(name string, closeClient func() error) error {
	if err := closeClient(); err != nil {
		return fmt.Errorf("close %s ent client: %w", name, err)
	}
	return nil
}
