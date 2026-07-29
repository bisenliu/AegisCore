package providers

import (
	"fmt"

	"entgo.io/ent/dialect"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

type entDriverPlugin interface {
	// WrapEntDriver 包装基础 Ent driver，并保持 Query/Exec/Tx 语义不变。
	WrapEntDriver(driver dialect.Driver) (dialect.Driver, error)
}

type entClientPlugin interface {
	// InstallEntClientPlugin 在 Ent client 上安装 hook 或 interceptor。
	InstallEntClientPlugin(client *ent.Client) error
}

// entPluginSet 按 Ent 构造生命周期拆分 driver 插件和 client 插件。
type entPluginSet struct {
	driverPlugins []entDriverPlugin
	clientPlugins []entClientPlugin
}

// newEntPlugins 将服务配置和可观测性 provider 转换为显式启用的 Ent 插件集合。
func newEntPlugins(
	settings serviceconfig.EntSettings,
	log *zap.Logger,
	metricsProvider *commonmetrics.Provider,
	tracingProvider *commontracing.Provider,
) (entPluginSet, error) {
	plugins := entPluginSet{}
	if settings.Plugins.SQLLog.Enabled {
		plugins.driverPlugins = append(plugins.driverPlugins, &entSQLLogPlugin{
			log:           logger.SQL(log),
			db:            primaryDatabaseResource,
			debugEnabled:  settings.Plugins.SQLLog.Debug,
			slowThreshold: settings.Plugins.SQLLog.SlowThreshold,
		})
	}
	if settings.Plugins.Tracing.Enabled && tracingProvider != nil {
		plugins.clientPlugins = append(plugins.clientPlugins, entTracingPlugin{
			tracer: tracingProvider.Tracer("github.com/aegiscore/user-service/internal/persistence/ent"),
		})
	}
	if settings.Plugins.Metrics.Enabled && metricsProvider != nil && metricsProvider.Enabled() {
		metrics, err := newEntQueryMetrics(metricsProvider)
		if err != nil {
			return entPluginSet{}, fmt.Errorf("create ent metrics plugin: %w", err)
		}
		plugins.clientPlugins = append(plugins.clientPlugins, entMetricsPlugin{metrics: metrics})
	}
	return plugins, nil
}
