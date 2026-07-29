package providers

import (
	"go.uber.org/fx"

	providerdatastore "github.com/aegiscore/user-service/internal/providers/datastore"
	providerobservability "github.com/aegiscore/user-service/internal/providers/observability"
	providersecurity "github.com/aegiscore/user-service/internal/providers/security"
	providertransport "github.com/aegiscore/user-service/internal/providers/transport"
)

// WiringModule 将用户服务级 provider 子模块接入 Fx，不主动执行运行时注册。
var WiringModule = fx.Module("user-service-providers-wiring",
	providerobservability.WiringModule,
	providersecurity.WiringModule,
	providerdatastore.WiringModule,
	providertransport.WiringModule,
)

// RuntimeModule 注册需要运行时主动执行的服务级生命周期和传输装配。
var RuntimeModule = fx.Module("user-service-providers-runtime",
	providerobservability.RuntimeModule,
	providertransport.RuntimeModule,
)

// Module 将用户服务级基础设施 provider 与运行时注册接入 Fx。
var Module = fx.Module("user-service-providers",
	WiringModule,
	RuntimeModule,
)
