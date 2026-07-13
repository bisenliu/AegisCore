package bootstrap

import (
	"net/http"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/logger"
	commontz "github.com/aegiscore/common/runtime/timezone"
	"github.com/aegiscore/common/validation"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authfeature "github.com/aegiscore/user-service/internal/features/auth"
	permissionfeature "github.com/aegiscore/user-service/internal/features/permission"
	rolefeature "github.com/aegiscore/user-service/internal/features/role"
	userfeature "github.com/aegiscore/user-service/internal/features/user"
	"github.com/aegiscore/user-service/internal/providers"
)

// Fx 分类注释统一使用“Fx 分类：<类别> - <职责>。”格式；新增装配项按以下规则归类：
// 基础运行时负责配置和进程初始化，资源负责外部连接与有状态运行组件，横切能力负责安全、校验和可观测性；
// Feature 基础设施负责实现 application port，Feature 应用负责用例与业务编排，传输负责协议入口，生命周期负责主动初始化或启停注册，开发工具仅服务构图与诊断。

// AppModule 组装 user-service 顶层模块、服务级 provider 和 HTTP server 生命周期。
var AppModule = fx.Module("aegiscore-user-services",
	// Fx 分类：基础运行时 - 跨 feature 的进程时区初始化。
	commontz.Module,
	// Fx 分类：横切能力 - 跨 feature 的输入校验能力。
	validation.Module,
	// Fx 分类：Feature 应用 - 各业务 feature 的完整内部装配。
	authfeature.Module,
	permissionfeature.Module,
	rolefeature.Module,
	userfeature.Module,
	// Fx 分类：基础运行时 - 汇总服务级资源、横切能力与传输装配。
	providers.Module,
	fx.Provide(
		// Fx 分类：传输 - 对外 HTTP server 及其生命周期 hook。
		NewHTTPServer,
	),
	fx.Invoke(
		// Fx 分类：生命周期 - 强制实例化 HTTP server 并注册启停 hook。
		func(*http.Server) {},
	),
)

// NewApp 构建包含共享配置、日志和服务模块的 user-service Fx 应用。
func NewApp(configPath string) *fx.App {
	return fx.New(
		// Fx 分类：基础运行时 - 应用启动输入。
		fx.Supply(serviceconfig.ConfigPath(configPath)),
		fx.Provide(
			// Fx 分类：基础运行时 - 配置加载、共享运行时配置和日志。
			serviceconfig.NewConfig,
			serviceconfig.NewRuntimeConfig,
			logger.NewLogger,
		),
		// Fx 分类：基础运行时 - user-service composition root。
		AppModule,
	)
}
