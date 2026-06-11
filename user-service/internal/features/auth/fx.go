package auth

import (
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	authpostgres "github.com/aegiscore/user-service/internal/features/auth/infrastructure/postgres"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	"go.uber.org/fx"
)

// Module wires the authentication feature application, transport, and infrastructure adapters.
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
		authdomain.NewRedisKeyBuilder,
		fx.Annotate(
			authredis.NewSessionStore,
			fx.As(new(authapplication.AuthSessionStore)),
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
