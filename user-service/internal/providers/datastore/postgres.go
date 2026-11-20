package datastore

import (
	"database/sql"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/datastore"
	platformtracing "github.com/aegiscore/platform/runtime/observability/tracing"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// PostgresParams 包含供应单个 PostgreSQL 连接池所需的 Fx 输入。
type PostgresParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Settings  serviceconfig.ResourceSettings
	Log       *zap.Logger
	Trace     *platformtracing.Provider
}

// ProvidePostgres 返回按调用方资源名选择配置并创建单个连接池的构造器。
//
// 资源名由 composition root 显式传入，constructor 不遍历配置 map，也不会连接未登记的数据库。
func ProvidePostgres(name string) func(PostgresParams) (*sql.DB, error) {
	return func(params PostgresParams) (*sql.DB, error) {
		if name == "" {
			return nil, fmt.Errorf("postgres resource name is required")
		}
		if params.Trace == nil {
			return nil, fmt.Errorf("postgres tracing provider is required")
		}
		cfg, ok := params.Settings.Postgres[name]
		if !ok {
			return nil, fmt.Errorf("postgres config %q not found", name)
		}
		db, err := datastore.NewPostgres(params.Lifecycle, params.Log, name, cfg, datastore.WithPostgresTracerProvider(params.Trace.OTelTracerProvider()))
		if err != nil {
			return nil, fmt.Errorf("provide postgres %s: %w", name, err)
		}
		return db, nil
	}
}
