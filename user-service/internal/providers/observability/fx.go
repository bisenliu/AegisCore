package observability

import (
	"go.uber.org/fx"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

// WiringModule 注册 user-service 可观测性 provider。
var WiringModule = fx.Module("user-service-providers-observability",
	fx.Provide(
		// Fx 分类：横切能力 - 服务级可观测性 provider。
		commonmetrics.NewMetricsProvider,
		commontracing.NewTracingProvider,
		// Fx 分类：横切能力 - 运行时健康检查聚合。
		ProvideHealthChecks,
	),
)

// RuntimeModule 注册需要启动期主动执行的可观测性装配。
var RuntimeModule = fx.Module("user-service-providers-observability-runtime",
	fx.Invoke(
		// Fx 分类：生命周期 - 启动期注册运行时依赖指标采集器。
		RegisterRuntimeDependencyMetrics,
	),
)
