package bootstrap

import (
	"net/http"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commontz "github.com/aegiscore/common/runtime/timezone"
	"github.com/aegiscore/common/validation"
	authfeature "github.com/aegiscore/user-service/internal/features/auth"
	userfeature "github.com/aegiscore/user-service/internal/features/user"
	"go.uber.org/fx"
)

// NewApp 构建包含共享配置、日志和服务模块的 user-service Fx 应用。
func NewApp(configPath string) *fx.App {
	return fx.New(
		fx.Supply(config.ConfigPath(configPath)),
		fx.Provide(
			config.NewConfig,
			logger.NewLogger,
		),
		AppModule,
	)
}

// AppModule 组装 user-service 运行时基础设施、仓储、服务、控制器、路由和 HTTP server。
var AppModule = fx.Module("aegiscore-user-services",
	commontz.Module,
	validation.Module,
	authfeature.Module,
	userfeature.Module,
	fx.Provide(
		ProvidePostgresPools,
		ProvideRedisClients,
		NewJWTService,
		ProvideEntClients,
		NewGinEngine,
		NewHTTPServer,
	),
	fx.Invoke(
		RegisterRoutes,
		// 确保 HTTP 服务器被实例化并将其生命周期 Hook 注册到 Fx 中
		func(*http.Server) {},
	),
)
