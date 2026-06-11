package user

import (
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userpostgres "github.com/aegiscore/user-service/internal/features/user/infra/postgres"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"go.uber.org/fx"
)

// Module wires the user profile feature application, transport, and infrastructure adapters.
var Module = fx.Module("feature-user",
	fx.Provide(
		fx.Annotate(
			userpostgres.NewUserStore,
			fx.As(new(userapplication.UserProfileStore)),
		),
		userapplication.NewUserService,
		userhttp.NewUserController,
	),
)
