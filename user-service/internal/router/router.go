package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	platformmw "github.com/aegiscore/platform/http/middleware"
	"github.com/aegiscore/platform/runtime/config"
	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
	platformauth "github.com/aegiscore/platform/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
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
	JWT                   platformauth.AccessTokenVerifier
	MetricsConfig         config.MetricsConfig
	HealthChecks          HealthChecks
	Metrics               *platformmetrics.Provider
	AnonymousRateLimiter  platformmw.RateLimiter
	UserRateLimiter       platformmw.RateLimiter
	TokenVersionValidator platformauth.TokenVersionValidator
	AccessDenylist        authapplication.AccessTokenDenylist
	Authorizer            permissionauthorization.Authorizer
	Auth                  *authhttp.AuthController
	Permission            *permissionhttp.PermissionController
	Role                  *rolehttp.RoleController
	User                  *userhttp.UserController
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
	rateLimitObserver, err := newRateLimitObserver(params.Log, params.Metrics)
	if err != nil {
		return fmt.Errorf("rate limit metrics: %w", err)
	}
	registerV1Routes(engine, params, rateLimitObserver)
	return nil
}

// registerV1Routes 挂载系统第一版业务接口（包含登录、权限、角色和用户管理）；普通业务接口必须登录并具备对应权限方可访问。
func registerV1Routes(engine *gin.Engine, params RouteParams, rateLimitObserver *rateLimitObserver) {
	v1 := engine.Group("/api/v1")

	publicAuth := v1.Group("/auth")
	publicAuth.Use(platformmw.RateLimit(rateLimitObserver.options("anonymous_auth", params.AnonymousRateLimiter, platformmw.IPRateLimitKey("anonymous-auth"))))
	authhttp.RegisterPublicRoutes(publicAuth, params.Auth)

	// 路由分层必须先认证再授权：auth 保护接口只需要登录态，permission/role/user 等业务接口还需 RBAC 授权。
	authenticated := v1.Group("")
	authenticated.Use(platformmw.AuthWithTokenVersionValidator(params.Log, params.JWT, params.TokenVersionValidator))
	authenticated.Use(authhttp.AccessTokenDenylist(params.Log, params.AccessDenylist))
	authenticated.Use(platformmw.RateLimit(rateLimitObserver.options("authenticated_api", params.UserRateLimiter, platformmw.UserIDRateLimitKey("authenticated-api"))))

	authhttp.RegisterProtectedRoutes(authenticated.Group("/auth"), params.Auth)

	authorized := authenticated.Group("")
	authorized.Use(permissionhttp.Authorize(params.Authorizer))
	permissionhttp.RegisterRoutes(authorized.Group("/permissions"), params.Permission)
	rolehttp.RegisterRoleRoutes(authorized.Group("/roles"), params.Role)
	rolehttp.RegisterUserRoleRoutes(authorized.Group("/users"), params.Role)
	userhttp.RegisterRoutes(authorized.Group("/users"), params.User)
}

// validateSecurityRouteDependencies 在启动前严密检查用户登录校验、权限判定以及各业务控制器是否均已就绪，缺一不可。
func validateSecurityRouteDependencies(params RouteParams) error {
	if params.TokenVersionValidator == nil {
		return fmt.Errorf("token version validator is required")
	}
	if params.AccessDenylist == nil {
		return fmt.Errorf("access token denylist is required")
	}
	if params.Authorizer == nil {
		return fmt.Errorf("rbac authorizer is required")
	}
	if params.Auth == nil {
		return fmt.Errorf("auth controller is required")
	}
	if params.Permission == nil {
		return fmt.Errorf("permission controller is required")
	}
	if params.Role == nil {
		return fmt.Errorf("role controller is required")
	}
	if params.User == nil {
		return fmt.Errorf("user controller is required")
	}
	return nil
}
