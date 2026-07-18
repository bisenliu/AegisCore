package router

import (
	"fmt"
	"sort"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	commonmw "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commonauth "github.com/aegiscore/common/security/auth"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
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
	PublicRoutes          []PublicRouteRegistrar
	AuthenticatedRoutes   []AuthenticatedRouteRegistrar
	AuthorizedRoutes      []AuthorizedRouteRegistrar
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

	for _, registrar := range sortedRouteRegistrars(params.PublicRoutes) {
		registrar.RegisterPublicRoutes(v1)
	}

	// 路由分层必须先认证再授权：auth 保护接口只需要登录态，permission/role/user 等业务接口还需 RBAC 授权。
	authenticated := v1.Group("")
	authenticated.Use(commonmw.AuthWithTokenVersionValidator(params.Log, params.JWT, params.TokenVersionValidator))

	for _, registrar := range sortedRouteRegistrars(params.AuthenticatedRoutes) {
		registrar.RegisterAuthenticatedRoutes(authenticated)
	}

	authorized := authenticated.Group("")
	authorized.Use(permissionhttp.Authorize(params.Authorizer))
	for _, registrar := range sortedRouteRegistrars(params.AuthorizedRoutes) {
		registrar.RegisterAuthorizedRoutes(authorized)
	}
}

func validateSecurityRouteDependencies(params RouteParams) error {
	if params.TokenVersionValidator == nil {
		return fmt.Errorf("token version validator is required")
	}
	if params.Authorizer == nil {
		return fmt.Errorf("rbac authorizer is required")
	}
	if err := requireRouteRegistrars("public route registrar", params.PublicRoutes, []string{"auth"}); err != nil {
		return err
	}
	if err := requireRouteRegistrars("authenticated route registrar", params.AuthenticatedRoutes, []string{"auth"}); err != nil {
		return err
	}
	return requireRouteRegistrars("authorized route registrar", params.AuthorizedRoutes, []string{"permission", "role", "user"})
}

type routeRegistrar interface {
	RouteKey() string
}

func sortedRouteRegistrars[T routeRegistrar](registrars []T) []T {
	sorted := append([]T(nil), registrars...)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].RouteKey() < sorted[j].RouteKey()
	})
	return sorted
}

func requireRouteRegistrars[T routeRegistrar](label string, registrars []T, requiredKeys []string) error {
	found := make(map[string]struct{}, len(registrars))
	for _, registrar := range registrars {
		if any(registrar) == nil {
			continue
		}
		found[registrar.RouteKey()] = struct{}{}
	}
	for _, key := range requiredKeys {
		if _, ok := found[key]; !ok {
			return fmt.Errorf("%s %s is required", label, key)
		}
	}
	return nil
}
