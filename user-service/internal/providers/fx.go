package providers

import (
	"go.uber.org/fx"

	providerdatastore "github.com/aegiscore/user-service/internal/providers/datastore"
	providerobservability "github.com/aegiscore/user-service/internal/providers/observability"
	providersecurity "github.com/aegiscore/user-service/internal/providers/security"
	providertransport "github.com/aegiscore/user-service/internal/providers/transport"
)

// WiringModule 汇总声明用户服务底层的基础设施组件（包括监控、安全、数据库、传输等），仅声明依赖关系，不立即触发运行时启动动作。
var WiringModule = fx.Module("user-service-providers-wiring",
	providerobservability.WiringModule,
	providersecurity.WiringModule,
	providerdatastore.WiringModule,
	providertransport.WiringModule,
)

// RuntimeModule 注册服务底层需要在系统启动时立即激活的运行时任务与事件监听。
var RuntimeModule = fx.Module("user-service-providers-runtime",
	providerobservability.RuntimeModule,
	providertransport.RuntimeModule,
)

// Module 将用户服务的组件声明与运行时启动逻辑统一打包为一个完整的底层模块。
var Module = fx.Module("user-service-providers",
	WiringModule,
	RuntimeModule,
)
