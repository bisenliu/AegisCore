package datastore

import (
	"fmt"

	"entgo.io/ent/dialect"
	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/logger"
	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
	platformtracing "github.com/aegiscore/platform/runtime/observability/tracing"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

type entDriverPlugin interface {
	// WrapEntDriver 为底层数据库驱动添加拦截能力（如慢查询监控、链路追踪），同时保证原有的 SQL 查询、执行与事务行为完全不受影响。
	WrapEntDriver(name string, driver dialect.Driver) (dialect.Driver, error)
}

type entClientPlugin interface {
	// InstallEntClientPlugin 为数据库操作客户端安装业务拦截插件或钩子函数。
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
	metricsProvider *platformmetrics.Provider,
	tracingProvider *platformtracing.Provider,
) (entPluginSet, error) {
	plugins := entPluginSet{}
	if settings.Plugins.SQLLog.Enabled {
		plugins.driverPlugins = append(plugins.driverPlugins, &entSQLLogPlugin{
			log:           logger.SQL(log),
			debugEnabled:  serviceconfig.DefaultEntSQLLogDebug,
			slowThreshold: serviceconfig.DefaultEntSlowQueryThreshold,
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
