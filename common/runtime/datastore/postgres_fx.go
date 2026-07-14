package datastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
)

// ProvideNamedPostgres 通过 Fx 名称到配置 key 的映射提供一个具名 PostgreSQL 连接池。
func ProvideNamedPostgres(fxName string, configKey string) fx.Option {
	// Fx 分类：资源 - 通用具名 PostgreSQL provider 工厂。
	return fx.Provide(fx.Annotate(
		func(lc fx.Lifecycle, configs resources.PostgresConfigs, log *zap.Logger) (*sql.DB, error) {
			return NewPostgres(lc, configs, log, configKey)
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, fxName)),
	))
}

// NewPostgres 创建一个具名 PostgreSQL 连接池并注册生命周期 hook。
func NewPostgres(lc fx.Lifecycle, configs resources.PostgresConfigs, log *zap.Logger, name string) (*sql.DB, error) {
	dbs, err := NewPostgresPools(lc, configs, log, name)
	if err != nil {
		return nil, err
	}
	return dbs[name], nil
}

// NewPostgresPools 创建多个具名 PostgreSQL 连接池并注册一个共享生命周期 hook。
func NewPostgresPools(lc fx.Lifecycle, configs resources.PostgresConfigs, log *zap.Logger, names ...string) (map[string]*sql.DB, error) {
	return newPostgresPools(lc, configs, log, OpenPostgres, names...)
}

type postgresOpener func(name string, cfg resources.PostgresConfig) (*sql.DB, error)

func newPostgresPools(lc fx.Lifecycle, configs resources.PostgresConfigs, log *zap.Logger, opener postgresOpener, names ...string) (map[string]*sql.DB, error) {
	dbs := make(map[string]*sql.DB, len(names))
	for _, name := range names {
		dbCfg, ok := configs[name]
		if !ok {
			// 关闭缺失配置前已打开的连接池，避免部分启动失败时泄漏资源。
			return nil, errors.Join(
				fmt.Errorf("postgres config %q not found", name),
				closePostgresPools(dbs),
			)
		}
		dbCfg.ApplyDefaults()
		db, err := opener(name, dbCfg)
		if err != nil {
			// 保留初始化错误，同时报告已创建连接池的清理失败。
			return nil, errors.Join(err, closePostgresPools(dbs))
		}
		dbs[name] = db
	}
	registerDBLifecycle(lc, log, dbs, names)
	return dbs, nil
}

func closePostgresPools(dbs map[string]*sql.DB) error {
	errs := make([]error, 0, len(dbs))
	for name, db := range dbs {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close postgres %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

func registerDBLifecycle(lc fx.Lifecycle, log *zap.Logger, dbs map[string]*sql.DB, names []string) {
	postgresLog := logger.NamedComponent(log, "postgres", "postgres")
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for _, name := range names {
				db := dbs[name]
				pingCtx, cancel := context.WithTimeout(ctx, resources.DefaultPostgresPingTimeout())
				if err := db.PingContext(pingCtx); err != nil {
					cancel()
					return errors.Join(
						fmt.Errorf("ping postgres %s: %w", name, err),
						closePostgresPools(dbs),
					)
				}
				cancel()
				logger.WithContext(ctx, postgresLog).Info("postgres connected", zap.String(logger.ResourceField, name))
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			errs := make([]error, 0, len(names))
			// 按声明顺序关闭所有连接池并聚合错误，避免遇到第一个错误就提前停止。
			for _, name := range names {
				if err := dbs[name].Close(); err != nil {
					errs = append(errs, fmt.Errorf("close postgres %s: %w", name, err))
					continue
				}
				logger.WithContext(ctx, postgresLog).Info("postgres closed", zap.String(logger.ResourceField, name))
			}
			return errors.Join(errs...)
		},
	})
}
