package bootstrap

import (
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/aegiscore/user-services/internal/router"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type RegisterRouteParams struct {
	fx.In

	Config         *config.Config
	Log            *zap.Logger
	Engine         *gin.Engine
	JWT            *auth.JWTService
	AuthSessions   repository.AuthSessionRepository `optional:"true"`
	AuthController *controller.AuthController
	UserController *controller.UserController
}

func RegisterRoutes(params RegisterRouteParams) {
	router.RegisterUserServiceHTTPRoutes(params.Engine, router.RouteParams{
		Environment:           params.Config.App.Environment,
		Log:                   params.Log,
		JWT:                   params.JWT,
		AuthConfig:            params.Config.Auth,
		TokenVersionValidator: params.AuthSessions,
		AuthController:        params.AuthController,
		UserController:        params.UserController,
	})
}
