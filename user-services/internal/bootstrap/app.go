package bootstrap

import (
	"net/http"

	"github.com/aegiscore/common/runtime/configfx"
	"github.com/aegiscore/common/runtime/loggerfx"
	commontz "github.com/aegiscore/common/runtime/timezone"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/auth"
	authredis "github.com/aegiscore/user-services/internal/auth/store/redis"
	"github.com/aegiscore/user-services/internal/user"
	userpostgres "github.com/aegiscore/user-services/internal/user/store/postgres"
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
		fx.Annotate(
			userpostgres.NewUserStore,
			fx.As(new(user.UserProfileStore)),
		),
		fx.Annotate(
			userpostgres.NewUserStore,
			fx.As(new(auth.UserCredentialStore)),
		),
		fx.Annotate(
			userpostgres.NewUserStore,
			fx.As(new(auth.UserTokenVersionStore)),
		),
		auth.NewRedisKeyBuilder,
		fx.Annotate(
			authredis.NewSessionStore,
			fx.As(new(auth.AuthSessionStore)),
		),
		auth.NewTokenVersionValidator,
		auth.NewAuthService,
		user.NewUserService,
		auth.NewAuthController,
		user.NewUserController,
		NewGinEngine,
		NewHTTPServer,
	),
	fx.Invoke(
		RegisterRoutes,
		// 确保 HTTP 服务器被实例化并将其生命周期 Hook 注册到 Fx 中
		func(*http.Server) {},
	),
)
