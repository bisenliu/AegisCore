package providers

import (
	"go.uber.org/fx"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

// Module 将用户服务级基础设施 provider 接入 Fx。
var Module = fx.Module("user-service-providers",
	fx.Provide(
		// Fx 分类：横切能力 - 服务级可观测性 provider。
		commonmetrics.NewFxProvider,
		commontracing.NewFxProvider,
		// Fx 分类：横切能力 - 服务级认证与密码安全能力。
		NewPasswordService,
		// Fx 分类：资源 - 服务拥有的 PostgreSQL 与 Redis 客户端。
		ProvidePostgresPools,
		ProvideRedisClients,
		// Fx 分类：横切能力 - 服务级 JWT 签发与校验能力。
		NewJWTService,
		// Fx 分类：资源 - 服务拥有的 Ent 客户端。
		ProvideEntClients,
		// Fx 分类：横切能力 - 运行时健康检查聚合。
		ProvideHealthChecks,
		// Fx 分类：传输 - Gin 引擎及 HTTP 中间件链。
		NewGinEngine,
	),
	fx.Invoke(
		// Fx 分类：生命周期 - 启动期注册运行时依赖指标采集器。
		RegisterRuntimeDependencyMetrics,
		// Fx 分类：传输 - 启动期完成 HTTP 路由总装。
		RegisterRoutes,
	),
)
