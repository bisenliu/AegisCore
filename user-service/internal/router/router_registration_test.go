package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	commonvalidation "github.com/aegiscore/common/validation"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

func TestRegisterUserServiceHTTPRoutesRegistersCurrentRouteGraph(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(openAPIEnabledEnv, "true")

	engine := gin.New()
	authorizer := &routerRegistrationAuthorizer{allowed: false}
	params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
		metrics:                     metricsRouteConfig(true, "/internal/metrics"),
		pprof:                       config.PprofConfig{Enabled: true, BasePath: "/internal/debug/pprof"},
		authorizer:                  authorizer,
		includePermissionController: true,
		includeRoleController:       true,
	})

	require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

	routes := collectRouterRegistrationRoutes(engine)
	requireRouterRoutesContain(t, routes, append(routerRegistrationRuntimeRoutes(), routerRegistrationV1Routes()...))
	requireRouterRoutesAbsent(t, routes, []routerRegisteredRoute{
		{method: http.MethodGet, path: "/metrics"},
		{method: http.MethodGet, path: "/debug/pprof"},
		{method: http.MethodGet, path: "/debug/pprof/*profile"},
		{method: http.MethodPost, path: "/api/auth/login"},
		{method: http.MethodPost, path: "/v1/auth/login"},
		{method: http.MethodGet, path: "/api/users"},
		{method: http.MethodGet, path: "/v1/users"},
	})

	publicAuth := executeRouterRegistrationRequest(engine, http.MethodPost, "/api/v1/auth/login", "", "")
	require.Equal(t, http.StatusBadRequest, publicAuth.Code, "body=%s", publicAuth.Body.String())

	protectedAuth := executeRouterRegistrationRequest(engine, http.MethodPost, "/api/v1/auth/logout", "", "")
	require.Equal(t, http.StatusUnauthorized, protectedAuth.Code, "body=%s", protectedAuth.Body.String())

	accessToken := signRouterRegistrationAccessToken(t, params.JWT)
	for _, route := range []routerRegisteredRoute{
		{method: http.MethodGet, path: "/api/v1/users"},
		{method: http.MethodGet, path: "/api/v1/permissions"},
		{method: http.MethodGet, path: "/api/v1/roles"},
	} {
		recorder := executeRouterRegistrationRequest(engine, route.method, route.path, accessToken, "")
		require.Equal(t, http.StatusForbidden, recorder.Code, "route=%s body=%s", routerRegistrationRouteKey(route), recorder.Body.String())
	}
	require.Equal(t, 3, authorizer.calls)
}

func TestRegisterV1RoutesSkipsNilOptionalControllers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
		includePermissionController: false,
		includeRoleController:       false,
	})

	registerV1Routes(engine, params)

	routes := collectRouterRegistrationRoutes(engine)
	requireRouterRoutesContain(t, routes, []routerRegisteredRoute{
		{method: http.MethodPost, path: "/api/v1/auth/login"},
		{method: http.MethodPost, path: "/api/v1/auth/logout"},
		{method: http.MethodGet, path: "/api/v1/users"},
		{method: http.MethodPost, path: "/api/v1/users"},
		{method: http.MethodGet, path: "/api/v1/users/:user_id"},
	})
	requireRouterRoutesAbsent(t, routes, append(routerRegistrationPermissionRoutes(), routerRegistrationRoleRoutes()...))
}

func TestRegisterUserServiceHTTPRoutesReturnsMetricsConfigError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(openAPIEnabledEnv, "true")

	engine := gin.New()
	params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
		metrics: metricsRouteConfig(true, "/api/v1/metrics"),
		pprof:   config.PprofConfig{Enabled: true, BasePath: "/internal/debug/pprof"},
	})

	err := RegisterUserServiceHTTPRoutes(engine, params)
	require.ErrorIs(t, err, ErrInvalidMetricsPath)
}

func TestRegisterUserServiceHTTPRoutesHonorsPprofSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(openAPIEnabledEnv, "true")

	t.Run("disabled does not register default path", func(t *testing.T) {
		engine := gin.New()
		params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
			metrics: metricsRouteConfig(false, "/metrics"),
			pprof:   config.PprofConfig{Enabled: false, BasePath: "/debug/pprof"},
		})

		require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

		routes := collectRouterRegistrationRoutes(engine)
		requireRouterRoutesAbsent(t, routes, []routerRegisteredRoute{
			{method: http.MethodGet, path: "/debug/pprof"},
			{method: http.MethodGet, path: "/debug/pprof/*profile"},
		})
	})

	t.Run("enabled registers only configured base path", func(t *testing.T) {
		engine := gin.New()
		params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
			metrics: metricsRouteConfig(false, "/metrics"),
			pprof:   config.PprofConfig{Enabled: true, BasePath: "/internal/debug/pprof"},
		})

		require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

		routes := collectRouterRegistrationRoutes(engine)
		requireRouterRoutesContain(t, routes, []routerRegisteredRoute{
			{method: http.MethodGet, path: "/internal/debug/pprof"},
			{method: http.MethodGet, path: "/internal/debug/pprof/*profile"},
		})
		requireRouterRoutesAbsent(t, routes, []routerRegisteredRoute{
			{method: http.MethodGet, path: "/debug/pprof"},
			{method: http.MethodGet, path: "/debug/pprof/*profile"},
		})
	})
}

type routerRegisteredRoute struct {
	method string
	path   string
}

type routerRegistrationRouteOptions struct {
	metrics                     config.MetricsConfig
	pprof                       config.PprofConfig
	authorizer                  permissionauthorization.Authorizer
	includePermissionController bool
	includeRoleController       bool
}

type routerRegistrationAuthorizer struct {
	allowed bool
	calls   int
}

func (a *routerRegistrationAuthorizer) Enforce(context.Context, string, string, string) (bool, error) {
	a.calls++
	return a.allowed, nil
}

func newRouterRegistrationRouteParams(t *testing.T, opts routerRegistrationRouteOptions) RouteParams {
	t.Helper()
	validator := newRouterRegistrationValidator(t)
	authCfg := config.AuthConfig{
		JWT: config.JWTConfig{
			Secret:         "router-registration-secret",
			AccessTokenTTL: time.Minute,
		},
	}
	metricsCfg := opts.metrics
	if metricsCfg.Path == "" {
		metricsCfg = metricsRouteConfig(false, "/metrics")
	}
	var permissionController *permissionhttp.PermissionController
	if opts.includePermissionController {
		permissionController = permissionhttp.NewPermissionController(nil, nil, validator)
	}
	var roleController *rolehttp.RoleController
	if opts.includeRoleController {
		roleController = rolehttp.NewRoleController(nil, nil, validator)
	}

	return RouteParams{
		ServiceName:   "aegiscore-user-service-test",
		Environment:   "test",
		Log:           zap.NewNop(),
		JWT:           commonauth.NewJWTService(authCfg),
		HTTPConfig:    config.HTTPConfig{Pprof: opts.pprof},
		MetricsConfig: metricsCfg,
		Metrics:       newRouterTestMetricsProvider(t, metricsCfg.Enabled, metricsCfg.Path),
		Authorizer:    opts.authorizer,
		AuthController: authhttp.NewAuthController(authhttp.AuthControllerParams{
			Validator: validator,
		}),
		PermissionController: permissionController,
		RoleController:       roleController,
		UserController:       userhttp.NewUserController(nil, nil, validator),
	}
}

func newRouterRegistrationValidator(t *testing.T) *commonvalidation.Validator {
	t.Helper()
	validator, err := commonvalidation.NewDefault()
	require.NoError(t, err)
	return validator
}

func collectRouterRegistrationRoutes(engine *gin.Engine) map[string]gin.RouteInfo {
	routes := make(map[string]gin.RouteInfo, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[routerRegistrationRouteKey(routerRegisteredRoute{method: route.Method, path: route.Path})] = route
	}
	return routes
}

func requireRouterRoutesContain(t *testing.T, routes map[string]gin.RouteInfo, expected []routerRegisteredRoute) {
	t.Helper()
	for _, route := range expected {
		require.Contains(t, routes, routerRegistrationRouteKey(route), "registered routes: %s", strings.Join(routerRegistrationRouteKeys(routes), ", "))
	}
}

func requireRouterRoutesAbsent(t *testing.T, routes map[string]gin.RouteInfo, expected []routerRegisteredRoute) {
	t.Helper()
	for _, route := range expected {
		require.NotContains(t, routes, routerRegistrationRouteKey(route), "registered routes: %s", strings.Join(routerRegistrationRouteKeys(routes), ", "))
	}
}

func routerRegistrationRouteKeys(routes map[string]gin.RouteInfo) []string {
	keys := make([]string, 0, len(routes))
	for key := range routes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func routerRegistrationRouteKey(route routerRegisteredRoute) string {
	return route.method + " " + route.path
}

func executeRouterRegistrationRequest(engine *gin.Engine, method string, path string, accessToken string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+accessToken)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func signRouterRegistrationAccessToken(t *testing.T, jwt *commonauth.JWTService) string {
	t.Helper()
	token, err := jwt.SignAccessToken(commonauth.SignInput{
		UserID:       uuid.NewString(),
		TokenVersion: 1,
		SessionID:    uuid.NewString(),
		TTL:          time.Minute,
	})
	require.NoError(t, err)
	return token
}

func routerRegistrationRuntimeRoutes() []routerRegisteredRoute {
	return []routerRegisteredRoute{
		{method: http.MethodGet, path: "/livez"},
		{method: http.MethodGet, path: "/readyz"},
		{method: http.MethodGet, path: "/startupz"},
		{method: http.MethodGet, path: openAPIJSONPath},
		{method: http.MethodGet, path: "/openapi/*any"},
		{method: http.MethodGet, path: "/docs"},
		{method: http.MethodGet, path: "/api-docs"},
		{method: http.MethodGet, path: "/internal/metrics"},
		{method: http.MethodGet, path: "/internal/debug/pprof"},
		{method: http.MethodGet, path: "/internal/debug/pprof/*profile"},
	}
}

func routerRegistrationV1Routes() []routerRegisteredRoute {
	routes := []routerRegisteredRoute{
		{method: http.MethodPost, path: "/api/v1/auth/login"},
		{method: http.MethodPost, path: "/api/v1/auth/refresh"},
		{method: http.MethodPost, path: "/api/v1/auth/change-password"},
		{method: http.MethodPost, path: "/api/v1/auth/logout"},
		{method: http.MethodPost, path: "/api/v1/auth/logout-all"},
		{method: http.MethodGet, path: "/api/v1/users"},
		{method: http.MethodPost, path: "/api/v1/users"},
		{method: http.MethodGet, path: "/api/v1/users/:user_id"},
	}
	routes = append(routes, routerRegistrationPermissionRoutes()...)
	routes = append(routes, routerRegistrationRoleRoutes()...)
	return routes
}

func routerRegistrationPermissionRoutes() []routerRegisteredRoute {
	return []routerRegisteredRoute{
		{method: http.MethodGet, path: "/api/v1/permissions"},
		{method: http.MethodPost, path: "/api/v1/permissions"},
		{method: http.MethodGet, path: "/api/v1/permissions/route-diff"},
		{method: http.MethodGet, path: "/api/v1/permissions/users/:user_id/effective"},
		{method: http.MethodGet, path: "/api/v1/permissions/:permission_id"},
		{method: http.MethodPut, path: "/api/v1/permissions/:permission_id"},
		{method: http.MethodPost, path: "/api/v1/permissions/:permission_id/enable"},
		{method: http.MethodPost, path: "/api/v1/permissions/:permission_id/disable"},
	}
}

func routerRegistrationRoleRoutes() []routerRegisteredRoute {
	return []routerRegisteredRoute{
		{method: http.MethodGet, path: "/api/v1/roles"},
		{method: http.MethodPost, path: "/api/v1/roles"},
		{method: http.MethodGet, path: "/api/v1/roles/:role_id"},
		{method: http.MethodPatch, path: "/api/v1/roles/:role_id"},
		{method: http.MethodPatch, path: "/api/v1/roles/:role_id/status"},
		{method: http.MethodGet, path: "/api/v1/roles/:role_id/permissions"},
		{method: http.MethodPut, path: "/api/v1/roles/:role_id/permissions"},
		{method: http.MethodPost, path: "/api/v1/roles/:role_id/permissions"},
		{method: http.MethodDelete, path: "/api/v1/roles/:role_id/permissions/:permission_id"},
		{method: http.MethodGet, path: "/api/v1/users/:user_id/roles"},
		{method: http.MethodPut, path: "/api/v1/users/:user_id/roles"},
		{method: http.MethodPost, path: "/api/v1/users/:user_id/roles"},
		{method: http.MethodDelete, path: "/api/v1/users/:user_id/roles/:role_id"},
	}
}
