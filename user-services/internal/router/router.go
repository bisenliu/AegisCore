package router

import (
	commonmw "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RouteParams 包含挂载用户服务 HTTP 路由所需的依赖。
type RouteParams struct {
	Environment           string
	Log                   *zap.Logger
	JWT                   *auth.JWTService
	AuthConfig            config.AuthConfig
	TokenVersionValidator commonmw.TokenVersionValidator
	AuthController        *controller.AuthController
	UserController        *controller.UserController
}

// RegisterUserServiceHTTPRoutes 挂载系统、Swagger、认证和用户 API 路由。
func RegisterUserServiceHTTPRoutes(engine *gin.Engine, params RouteParams) {
	registerSystemRoutes(engine)
	RegisterSwagger(engine, params.Environment)
	registerV1Routes(engine, params)
}

func registerV1Routes(engine *gin.Engine, params RouteParams) {
	v1 := engine.Group("/api/v1")

	registerPublicAuthRoutes(v1.Group("/auth"), params.AuthController)

	authenticated := v1.Group("")
	authenticated.Use(commonmw.AuthWithTokenVersionValidator(params.Log, params.JWT, params.AuthConfig, params.TokenVersionValidator))

	registerProtectedAuthRoutes(authenticated.Group("/auth"), params.AuthController)

	// Mount future Casbin authorization middleware on this group after authentication.
	authorized := authenticated.Group("")
	registerUserRoutes(authorized.Group("/users"), params.UserController)
}
