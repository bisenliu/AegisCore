package auth

import (
	authapp "github.com/aegiscore/user-services/internal/features/auth/app"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	authpostgres "github.com/aegiscore/user-services/internal/features/auth/infra/postgres"
	authredis "github.com/aegiscore/user-services/internal/features/auth/infra/redis"
	authhttp "github.com/aegiscore/user-services/internal/features/auth/transport/http"
	"go.uber.org/fx"
)

// Module wires the authentication feature application, transport, and infrastructure adapters.
var Module = fx.Module("feature-auth",
	fx.Provide(
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapp.UserCredentialStore)),
		),
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapp.UserTokenVersionStore)),
		),
		authdomain.NewRedisKeyBuilder,
		fx.Annotate(
			authredis.NewSessionStore,
			fx.As(new(authapp.AuthSessionStore)),
		),
		authapp.NewTokenVersionValidator,
		authapp.NewAuthService,
		authhttp.NewAuthController,
	),
)
