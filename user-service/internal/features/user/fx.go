package user

import (
	"go.uber.org/fx"

	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	userpostgres "github.com/aegiscore/user-service/internal/features/user/infrastructure/postgres"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

// Module 组装用户资料 feature 的应用层、传输层和基础设施适配器。
var Module = fx.Module("feature-user",
	fx.Provide(
		fx.Annotate(
			userpostgres.NewUserStore,
			fx.As(new(userapplication.UserProfileStore)),
		),
		usercommand.NewCreateUserService,
		userquery.NewUserQueryService,
		userhttp.NewUserController,
	),
)
