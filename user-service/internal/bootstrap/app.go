package bootstrap

import (
	"go.uber.org/fx"

	"github.com/aegiscore/platform/runtime/logger"
	platformtz "github.com/aegiscore/platform/runtime/timezone"
	"github.com/aegiscore/platform/validation"
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

// WiringModule 汇总声明整个用户服务所需的所有组件及其依赖关系（只声明关系，不立即启动后台任务或发起网络请求）。
var WiringModule = fx.Module("aegiscore-user-service-wiring",
	// Fx 分类：横切能力 - 跨 feature 的输入校验能力。
	validation.Module,
	// Fx 分类：Feature 应用 - 各业务 feature 的完整内部装配。
	authfeature.Module,
	permissionfeature.WiringModule,
	rolefeature.Module,
	userfeature.Module,
	// Fx 分类：基础运行时 - 汇总服务级资源、横切能力与传输 provider 装配。
	providers.WiringModule,
	fx.Provide(
		// Fx 分类：传输 - 对外 HTTP server 及其生命周期 hook。
		NewHTTPServer,
		// Fx 分类：开发工具 - 通过已解析配置控制的独立 pprof 诊断 server。
		NewPprofServer,
	),
)

// RuntimeModule 注册服务在启动运行时需要主动执行的初始化操作、路由监听与生命周期启停事件。
var RuntimeModule = fx.Module("aegiscore-user-service-runtime",
	// Fx 分类：基础运行时 - 跨 feature 的进程时区初始化。
	fx.Invoke(InitProcessRuntime),
	// Fx 分类：生命周期 - 启动期服务级 runtime 注册。
	providers.RuntimeModule,
	// Fx 分类：生命周期 - 权限 feature 的 RBAC 初始化和 watcher lifecycle。
	permissionfeature.LifecycleModule,
	// Fx 分类：生命周期 - 强制解析运行时 server 并注册启停 hook。
	fx.Invoke(registerRuntimeServers),
)

// registerRuntimeServers 触发 HTTP 服务与性能诊断服务的初始化，使它们随系统生命周期一同启动和关闭。
func registerRuntimeServers(_ *HTTPRuntime, _ *PprofRuntime) {}

// InitProcessRuntime 初始化 user-service 拥有的进程级 runtime 状态。
func InitProcessRuntime(settings serviceconfig.TimezoneSettings) error {
	return platformtz.Init(settings.Name)
}

// AppModule 汇总用户服务的全部组件装配与运行时启动逻辑，作为服务启动的唯一总入口。
var AppModule = fx.Module("aegiscore-user-service",
	WiringModule,
	RuntimeModule,
)

// AppOptions 从已解析的 service config 构建无配置 I/O 的基础 Fx options。
func AppOptions(cfg *serviceconfig.Config, additional ...fx.Option) []fx.Option {
	lifecycleCfg := cfg.Runtime.Lifecycle
	options := []fx.Option{
		// Fx 分类：基础运行时 - 将 DI 初始化期 panic 转换为 Fx 错误。
		fx.RecoverFromPanics(),
		// Fx 分类：基础运行时 - 根配置仅供 composition provider 派生共享 runtime 与窄 settings。
		fx.Supply(cfg),
		fx.Provide(
			serviceconfig.NewRuntimeConfig,
			serviceconfig.NewAuthSettings,
			serviceconfig.NewRBACSettings,
			serviceconfig.NewEntSettings,
			serviceconfig.NewRateLimitSettings,
			serviceconfig.NewHTTPSettings,
			serviceconfig.NewTimezoneSettings,
			serviceconfig.NewResourceSettings,
			// Fx 分类：基础运行时 - 结构化日志。
			logger.NewLogger,
		),
		// Fx 分类：基础运行时 - 将 Fx 自身事件接入结构化日志。
		fx.WithLogger(logger.NewFxEventLogger),
		// 为 App.Run、fxtest 和 timeout 查询提供默认预算；显式 App.Start/Stop 仍以调用方 context 为实际边界。
		fx.StartTimeout(lifecycleCfg.StartTimeout),
		fx.StopTimeout(lifecycleCfg.StopTimeout),
	}
	return append(options, additional...)
}

// NewApp 使用调用方已解析的配置构建 user-service Fx 应用。
func NewApp(cfg *serviceconfig.Config) *fx.App {
	return fx.New(AppOptions(
		cfg,
		// Fx 分类：基础运行时 - user-service composition root。
		AppModule,
	)...)
}
