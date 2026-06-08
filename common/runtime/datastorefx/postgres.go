package datastorefx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ProvideNamedPostgres 通过 Fx 名称到配置 key 的映射提供一个具名 PostgreSQL 连接池。
func ProvideNamedPostgres(fxName string, configKey string) fx.Option {
	return fx.Provide(fx.Annotate(
		func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*sql.DB, error) {
			return NewPostgres(lc, cfg, log, configKey)
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, fxName)),
	))
}

// NewPostgres 创建一个具名 PostgreSQL 连接池并注册生命周期 hook。
func NewPostgres(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, name string) (*sql.DB, error) {
	dbs, err := NewPostgresPools(lc, cfg, log, name)
	if err != nil {
		return nil, err
	}
	return dbs[name], nil
}

// NewPostgresPools 创建多个具名 PostgreSQL 连接池并注册一个共享生命周期 hook。
func NewPostgresPools(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, names ...string) (map[string]*sql.DB, error) {
	dbs := make(map[string]*sql.DB, len(names))
	dbCfgs := make(map[string]config.PostgresDBConfig, len(names))
	for _, name := range names {
		dbCfg, ok := cfg.PostgresDatabaseConfig(name)
		if !ok {
			// 关闭缺失配置前已打开的连接池，避免部分启动失败时泄漏资源。
			return nil, errors.Join(
				fmt.Errorf("postgres config %q not found", name),
				closePostgresPools(dbs),
			)
		}
		db, err := datastore.OpenPostgres(name, dbCfg)
		if err != nil {
			// 保留初始化错误，同时报告已创建连接池的清理失败。
			return nil, errors.Join(err, closePostgresPools(dbs))
		}
		dbs[name] = db
		dbCfgs[name] = dbCfg
	}
	registerDBLifecycle(lc, log, dbs, dbCfgs, names)
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

func registerDBLifecycle(lc fx.Lifecycle, log *zap.Logger, dbs map[string]*sql.DB, dbCfgs map[string]config.PostgresDBConfig, names []string) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for _, name := range names {
				db := dbs[name]
				cfg := dbCfgs[name]
				pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
				if err := db.PingContext(pingCtx); err != nil {
					cancel()
					return fmt.Errorf("ping postgres %s: %w", name, err)
				}
				cancel()
				logger.WithContext(log, ctx).Info("postgres connected", zap.String("name", name))
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
				logger.WithContext(log, ctx).Info("postgres closed", zap.String("name", name))
			}
			return errors.Join(errs...)
		},
	})
}
