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
		// Fx 分类：Feature 基础设施 - PostgreSQL user profile port adapter。
		fx.Annotate(
			userpostgres.NewUserStore,
			fx.ParamTags(`name:"primary_db"`),
			fx.As(new(userapplication.UserProfileStore)),
		),
		// Fx 分类：Feature 应用 - 用户资料命令与查询服务。
		usercommand.NewCreateUserService,
		userquery.NewUserQueryService,
		// Fx 分类：传输 - user HTTP controller。
		userhttp.NewUserController,
	),
)
