package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	commonmw "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

// RouteParams 包含挂载用户服务 HTTP 路由所需的依赖。
type RouteParams struct {
	ServiceName           string
	Environment           string
	Log                   *zap.Logger
	JWT                   *commonauth.JWTService
	AuthConfig            config.AuthConfig
	TokenVersionValidator commonauth.TokenVersionValidator
	AuthController        *authhttp.AuthController
	PermissionController  *permissionhttp.PermissionController
	UserController        *userhttp.UserController
}

// RegisterUserServiceHTTPRoutes 挂载健康检查、Swagger、认证和用户 API 路由。
func RegisterUserServiceHTTPRoutes(engine *gin.Engine, params RouteParams) {
	registerHealthRoutes(engine, params.ServiceName)
	RegisterSwagger(engine, params.Environment)
	registerV1Routes(engine, params)
}

func registerV1Routes(engine *gin.Engine, params RouteParams) {
	v1 := engine.Group("/api/v1")

	authhttp.RegisterPublicRoutes(v1.Group("/auth"), params.AuthController)

	authenticated := v1.Group("")
	authenticated.Use(commonmw.AuthWithTokenVersionValidator(params.Log, params.JWT, params.AuthConfig, params.TokenVersionValidator))

	authhttp.RegisterProtectedRoutes(authenticated.Group("/auth"), params.AuthController)

	// 未来 Casbin 授权中间件应在认证之后挂载到该分组。
	authorized := authenticated.Group("")
	if params.PermissionController != nil {
		permissionhttp.RegisterRoutes(authorized.Group("/permissions"), params.PermissionController)
	}
	userhttp.RegisterRoutes(authorized.Group("/users"), params.UserController)
}
