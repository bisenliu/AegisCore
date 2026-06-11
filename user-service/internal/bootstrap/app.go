package bootstrap

import (
	"net/http"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commontz "github.com/aegiscore/common/runtime/timezone"
	"github.com/aegiscore/common/validation"
	authfeature "github.com/aegiscore/user-service/internal/features/auth"
	userfeature "github.com/aegiscore/user-service/internal/features/user"
	"github.com/aegiscore/user-service/internal/providers"
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

// AppModule 组装 user-service 顶层模块、服务级 provider 和 HTTP server 生命周期。
var AppModule = fx.Module("aegiscore-user-services",
	commontz.Module,
	validation.Module,
	authfeature.Module,
	userfeature.Module,
	providers.Module,
	fx.Provide(
		NewHTTPServer,
	),
	fx.Invoke(
		// 确保 HTTP 服务器被实例化并将其生命周期 Hook 注册到 Fx 中
		func(*http.Server) {},
	),
)
