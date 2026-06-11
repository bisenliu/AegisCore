package auth

import (
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	authpostgres "github.com/aegiscore/user-service/internal/features/auth/infra/postgres"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infra/redis"
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
		authapplication.NewTokenVersionValidator,
		authapplication.NewAuthService,
		authhttp.NewAuthController,
	),
)
