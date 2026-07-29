package security

import (
	"go.uber.org/fx"

	"github.com/aegiscore/common/security/password"
)

// WiringModule 注册 user-service 安全 provider。
var WiringModule = fx.Module("user-service-providers-security",
	fx.Provide(
		// Fx 分类：横切能力 - 服务级认证与密码安全能力。
		password.NewService,
		// Fx 分类：横切能力 - 服务级 JWT 签发与校验能力。
		NewJWTService,
	),
)
