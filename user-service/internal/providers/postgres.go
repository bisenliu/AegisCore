package providers

import (
	"database/sql"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/datastore"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/resources"
)

// PrimaryDBParams 包含供应 user-service 主 PostgreSQL 连接池所需的 Fx 输入。
type PrimaryDBParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Settings  serviceconfig.ResourceSettings
	Log       *zap.Logger
	Trace     *commontracing.Provider
}

// NewPrimaryDB 显式选择并供应 user-service 主 PostgreSQL 连接池。
func NewPrimaryDB(params PrimaryDBParams) (*sql.DB, error) {
	cfg, ok := params.Settings.Postgres[resources.NamePrimaryDB]
	if !ok {
		return nil, fmt.Errorf("postgres config %q not found", resources.NamePrimaryDB)
	}
	return datastore.NewPostgres(params.Lifecycle, params.Log, resources.NamePrimaryDB, cfg, datastore.WithPostgresTracerProvider(params.Trace.OTelTracerProvider()))
}
