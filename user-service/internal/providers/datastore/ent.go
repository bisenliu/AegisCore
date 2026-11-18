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

// EntPlugins 是可安装到多个同 schema Ent client 的插件集合。
type EntPlugins struct {
	plugins entPluginSet
}

// nonClosingEntDriver 将 Ent client 的生命周期与由 Fx 管理的底层 SQL 连接池解耦。
type nonClosingEntDriver struct {
	dialect.Driver
}

// ProvideEntPlugins 按服务配置构造一次 Ent 插件。
//
// metrics collector 在这里注册，后续每个具名 Ent client 只复用该集合，避免按连接重复注册。
func ProvideEntPlugins(settings serviceconfig.EntSettings, log *zap.Logger, metrics *commonmetrics.Provider, tracing *commontracing.Provider) (EntPlugins, error) {
	if metrics == nil {
		return EntPlugins{}, fmt.Errorf("ent metrics provider is required")
	}
	if tracing == nil {
		return EntPlugins{}, fmt.Errorf("ent tracing provider is required")
	}
	plugins, err := newEntPlugins(settings, log, metrics, tracing)
	if err != nil {
		return EntPlugins{}, err
	}
	return EntPlugins{plugins: plugins}, nil
}

// ProvideEntClient 返回按调用方资源名包装单个 Ent client 的构造器。
//
// 返回函数使用普通参数，便于 composition root 只给 *sql.DB 加 name tag。
// 只有显式登记、且与本服务 Ent schema 相同的连接池才应调用该构造器。
func ProvideEntClient(name string) func(fx.Lifecycle, *zap.Logger, *sql.DB, EntPlugins) (*ent.Client, error) {
	return func(lc fx.Lifecycle, log *zap.Logger, db *sql.DB, plugins EntPlugins) (*ent.Client, error) {
		if name == "" {
			return nil, fmt.Errorf("ent resource name is required")
		}
		client, err := newEntClient(name, db, plugins.plugins)
		if err != nil {
			return nil, fmt.Errorf("provide ent client %s: %w", name, err)
		}
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				logger.WithContext(ctx, log).Info("closing ent client", zap.String("resource", name))
				if err := client.Close(); err != nil {
					return fmt.Errorf("close %s ent client: %w", name, err)
				}
				return nil
			},
		})
		return client, nil
	}
}

// newEntClient 创建数据库实体操作客户端，并自动挂载 SQL 执行耗时监控和慢查询日志。
func newEntClient(name string, db *sql.DB, plugins entPluginSet) (*ent.Client, error) {
	driver := newEntDriver(db)
	var err error
	for _, plugin := range plugins.driverPlugins {
		driver, err = plugin.WrapEntDriver(name, driver)
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

// newEntDriver 将底层数据库连接池包装为数据访问层所需的驱动组件。
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
