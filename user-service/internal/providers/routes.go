package providers

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"github.com/aegiscore/user-service/internal/router"
)

// RegisterRouteParams 包含挂载用户服务路由所需的依赖。
type RegisterRouteParams struct {
	fx.In

	Config *config.Config
	Log    *zap.Logger
	Engine *gin.Engine
	JWT    *commonauth.JWTService
	// TokenVersions 是可选依赖，使公开路由和测试可以在不提供撤销能力时挂载中间件。
	TokenVersions        commonauth.TokenVersionValidator `optional:"true"`
	AuthController       *authhttp.AuthController
	PermissionController *permissionhttp.PermissionController `optional:"true"`
	RoleController       *rolehttp.RoleController             `optional:"true"`
	UserController       *userhttp.UserController
}

// RegisterRoutes 将服务级 provider 依赖适配为 router 层路由注册参数。
func RegisterRoutes(params RegisterRouteParams) {
	router.RegisterUserServiceHTTPRoutes(params.Engine, router.RouteParams{
		ServiceName:           params.Config.App.Name,
		Environment:           params.Config.App.Environment,
		Log:                   params.Log,
		JWT:                   params.JWT,
		AuthConfig:            params.Config.Auth,
		TokenVersionValidator: params.TokenVersions,
		AuthController:        params.AuthController,
		PermissionController:  params.PermissionController,
		RoleController:        params.RoleController,
		UserController:        params.UserController,
	})
}
