package transport

import "go.uber.org/fx"

// WiringModule 注册 user-service HTTP transport provider。
var WiringModule = fx.Module("user-service-providers-transport",
	fx.Provide(
		// Fx 分类：基础运行时 - 显式设置 Gin 进程运行模式。
		ConfigureGinMode,
		// Fx 分类：横切能力 - 服务级 API 限流能力。
		NewAPIRateLimiters,
		// Fx 分类：传输 - Gin 引擎及 HTTP 中间件链。
		NewGinEngine,
	),
)

// RuntimeModule 注册需要启动期主动执行的 HTTP transport 装配。
var RuntimeModule = fx.Module("user-service-providers-transport-runtime",
	fx.Invoke(
		// Fx 分类：传输 - 启动期完成 HTTP 路由总装。
		RegisterRoutes,
	),
)
