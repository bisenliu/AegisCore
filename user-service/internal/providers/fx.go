package providers

import (
	"go.uber.org/fx"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/common/security/password"
)

// WiringModule 将用户服务级基础设施 provider 接入 Fx，不主动执行运行时注册。
var WiringModule = fx.Module("user-service-providers-wiring",
	fx.Provide(
		// Fx 分类：基础运行时 - 显式设置 Gin 进程运行模式。
		ConfigureGinMode,
		// Fx 分类：横切能力 - 服务级可观测性 provider。
		commonmetrics.NewMetricsProvider,
		commontracing.NewTracingProvider,
		// Fx 分类：横切能力 - 服务级认证与密码安全能力。
		password.NewService,
		// Fx 分类：资源 - 服务拥有的 PostgreSQL 与 Redis 客户端。
		fx.Annotate(NewPrimaryDB, fx.ResultTags(`name:"primary_db"`)),
		fx.Annotate(NewCacheRedis, fx.ResultTags(`name:"cache_redis"`)),
		// Fx 分类：横切能力 - 服务级 JWT 签发与校验能力。
		NewJWTService,
		// Fx 分类：资源 - 服务拥有的 Ent 客户端。
		ProvideEntClients,
		// Fx 分类：横切能力 - 运行时健康检查聚合。
		ProvideHealthChecks,
		// Fx 分类：传输 - Gin 引擎及 HTTP 中间件链。
		NewGinEngine,
	),
)

// RuntimeModule 注册需要运行时主动执行的服务级生命周期和传输装配。
var RuntimeModule = fx.Module("user-service-providers-runtime",
	fx.Invoke(
		// Fx 分类：生命周期 - 启动期注册运行时依赖指标采集器。
		RegisterRuntimeDependencyMetrics,
		// Fx 分类：传输 - 启动期完成 HTTP 路由总装。
		RegisterRoutes,
	),
)

// Module 将用户服务级基础设施 provider 与运行时注册接入 Fx。
var Module = fx.Module("user-service-providers",
	WiringModule,
	RuntimeModule,
)
