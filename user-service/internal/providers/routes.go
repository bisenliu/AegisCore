package providers

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commonauth "github.com/aegiscore/common/security/auth"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"github.com/aegiscore/user-service/internal/router"
)

// RegisterRouteParams 包含挂载用户服务路由所需的依赖。
type RegisterRouteParams struct {
	fx.In

	Config               *config.Config
	Log                  *zap.Logger
	Engine               *gin.Engine
	JWT                  commonauth.AccessTokenVerifier
	Health               router.HealthChecks
	Metrics              *commonmetrics.Provider
	TokenVersions        commonauth.TokenVersionValidator
	Authorizer           permissionauthorization.Authorizer
	AuthController       *authhttp.AuthController
	PermissionController *permissionhttp.PermissionController
	RoleController       *rolehttp.RoleController
	UserController       *userhttp.UserController
}

// RegisterRoutes 将服务级 provider 依赖适配为 router 层路由注册参数。
func RegisterRoutes(params RegisterRouteParams) error {
	return router.RegisterUserServiceHTTPRoutes(params.Engine, router.RouteParams{
		ServiceName:           params.Config.App.Name,
		Environment:           params.Config.App.Environment,
		Log:                   params.Log,
		JWT:                   params.JWT,
		MetricsConfig:         params.Config.Observability.Metrics,
		HealthChecks:          params.Health,
		Metrics:               params.Metrics,
		TokenVersionValidator: params.TokenVersions,
		Authorizer:            params.Authorizer,
		AuthController:        params.AuthController,
		PermissionController:  params.PermissionController,
		RoleController:        params.RoleController,
		UserController:        params.UserController,
	})
}
