package bootstrap

import (
	"net/http"

	"github.com/aegiscore/common/runtime/configfx"
	"github.com/aegiscore/common/runtime/loggerfx"
	commontz "github.com/aegiscore/common/runtime/timezone"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/repository/postgres"
	"github.com/aegiscore/user-services/internal/repository/redis"
	"github.com/aegiscore/user-services/internal/service"
	"go.uber.org/fx"
)

// NewApp 构建包含共享配置、日志和服务模块的 user-services Fx 应用。
func NewApp(configPath string) *fx.App {
	return fx.New(
		fx.Supply(configfx.ConfigPath(configPath)),
		fx.Provide(
			configfx.NewConfig,
			loggerfx.NewLogger,
		),
		AppModule,
	)
}

// AppModule 组装 user-services 运行时基础设施、仓储、服务、控制器、路由和 HTTP server。
var AppModule = fx.Module("aegiscore-user-services",
	commontz.Module,
	validation.Module,
	fx.Provide(
		ProvidePostgresPools,
		ProvideRedisClients,
		NewJWTService,
		ProvideEntClients,
		postgres.NewUserRepository,
		postgres.AsUserProfileRepository,
		postgres.AsUserCredentialRepository,
		postgres.AsUserTokenVersionRepository,
		service.NewRedisKeyBuilder,
		redis.NewAuthSessionRepository,
		service.NewTokenVersionValidator,
		service.NewAuthService,
		service.NewUserService,
		controller.NewAuthController,
		controller.NewUserController,
		NewGinEngine,
		NewHTTPServer,
	),
	fx.Invoke(
		RegisterRoutes,
		// 确保 HTTP 服务器被实例化并将其生命周期 Hook 注册到 Fx 中
		func(*http.Server) {},
	),
)
