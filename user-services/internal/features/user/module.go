package user

import (
	userapp "github.com/aegiscore/user-services/internal/features/user/app"
	userpostgres "github.com/aegiscore/user-services/internal/features/user/infra/postgres"
	userhttp "github.com/aegiscore/user-services/internal/features/user/transport/http"
	"go.uber.org/fx"
)

// Module wires the user profile feature application, transport, and infrastructure adapters.
var Module = fx.Module("feature-user",
	fx.Provide(
		fx.Annotate(
			userpostgres.NewUserStore,
			fx.As(new(userapp.UserProfileStore)),
		),
		userapp.NewUserService,
		userhttp.NewUserController,
	),
)
