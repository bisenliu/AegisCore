package bootstrap

import (
	"net/http"

	commoninfra "github.com/aegiscore/common/infrastructure"
	commontz "github.com/aegiscore/common/timezone"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/repository/postgres"
	"github.com/aegiscore/user-services/internal/repository/redis"
	"github.com/aegiscore/user-services/internal/service"
	"go.uber.org/fx"
)

func NewApp(configPath string) *fx.App {
	return fx.New(
		fx.Supply(commoninfra.ConfigPath(configPath)),
		fx.Provide(
			commoninfra.NewConfig,
			commoninfra.NewLogger,
		),
		UserServiceModule,
	)
}

var UserServiceModule = fx.Module("aegiscore-user-services",
	commontz.Module,
	validation.Module,
	fx.Provide(
		ProvidePostgresPools,
		ProvideRedisClients,
		NewJWTService,
		ProvideEntClients,
		postgres.NewUserRepository,
		redis.NewAuthSessionRepository,
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
