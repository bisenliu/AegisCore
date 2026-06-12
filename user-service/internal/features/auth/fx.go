package auth

import (
	"go.uber.org/fx"

	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authpostgres "github.com/aegiscore/user-service/internal/features/auth/infrastructure/postgres"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
)

// Module 组装认证功能的应用服务、HTTP 传输层和基础设施适配器。
var Module = fx.Module("feature-auth",
	fx.Provide(
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapplication.UserCredentialStore)),
		),
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapplication.UserTokenVersionStore)),
		),
		fx.Annotate(
			authredis.NewSessionStore,
			fx.As(new(authapplication.AuthSessionStore)),
		),
		fx.Annotate(
			authredis.NewSessionPurgePool,
			fx.As(new(authredis.PurgeTaskPool)),
			fx.ResultTags(`name:"auth_session_purge_pool"`),
		),
		authvalidators.NewValidator,
		authcommand.NewUseCaseDeps,
		authcommand.NewLoginUseCase,
		authcommand.NewRefreshTokenUseCase,
		authcommand.NewChangePasswordUseCase,
		authcommand.NewLogoutCurrentSessionUseCase,
		authcommand.NewLogoutAllSessionsUseCase,
		authhttp.NewAuthController,
	),
)
