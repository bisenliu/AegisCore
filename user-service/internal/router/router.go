package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	commonmw "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commonauth "github.com/aegiscore/common/security/auth"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

// RouteParams 包含挂载用户服务 HTTP 路由所需的依赖。
type RouteParams struct {
	ServiceName           string
	Environment           string
	Log                   *zap.Logger
	JWT                   commonauth.AccessTokenVerifier
	MetricsConfig         config.MetricsConfig
	HealthChecks          HealthChecks
	Metrics               *commonmetrics.Provider
	TokenVersionValidator commonauth.TokenVersionValidator
	Authorizer            permissionauthorization.Authorizer
	AuthController        *authhttp.AuthController
	PermissionController  *permissionhttp.PermissionController
	RoleController        *rolehttp.RoleController
	UserController        *userhttp.UserController
}

// RegisterUserServiceHTTPRoutes 挂载健康检查、OpenAPI、metrics、认证和用户 API 路由。
func RegisterUserServiceHTTPRoutes(engine *gin.Engine, params RouteParams) error {
	if err := validateSecurityRouteDependencies(params); err != nil {
		return err
	}
	registerHealthRoutes(engine, params.ServiceName, params.HealthChecks)
	RegisterOpenAPI(engine, params.Environment)
	if err := registerMetricsRoute(engine, MetricsRouteParams{Config: params.MetricsConfig, Provider: params.Metrics}); err != nil {
		return err
	}
	registerV1Routes(engine, params)
	return nil
}

func registerV1Routes(engine *gin.Engine, params RouteParams) {
	v1 := engine.Group("/api/v1")

	authhttp.RegisterPublicRoutes(v1.Group("/auth"), params.AuthController)

	// 路由分层必须先认证再授权：auth 保护接口只需要登录态，permission/role/user 等业务接口还需 RBAC 授权。
	authenticated := v1.Group("")
	authenticated.Use(commonmw.AuthWithTokenVersionValidator(params.Log, params.JWT, params.TokenVersionValidator))

	authhttp.RegisterProtectedRoutes(authenticated.Group("/auth"), params.AuthController)

	authorized := authenticated.Group("")
	authorized.Use(permissionhttp.Authorize(params.Authorizer))
	permissionhttp.RegisterRoutes(authorized.Group("/permissions"), params.PermissionController)
	rolehttp.RegisterRoleRoutes(authorized.Group("/roles"), params.RoleController)
	rolehttp.RegisterUserRoleRoutes(authorized.Group("/users"), params.RoleController)
	userhttp.RegisterRoutes(authorized.Group("/users"), params.UserController)
}

func validateSecurityRouteDependencies(params RouteParams) error {
	if params.TokenVersionValidator == nil {
		return fmt.Errorf("token version validator is required")
	}
	if params.Authorizer == nil {
		return fmt.Errorf("rbac authorizer is required")
	}
	if params.PermissionController == nil {
		return fmt.Errorf("permission controller is required")
	}
	if params.RoleController == nil {
		return fmt.Errorf("role controller is required")
	}
	return nil
}
