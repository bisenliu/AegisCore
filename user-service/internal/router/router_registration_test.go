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
	jwtv5 "github.com/golang-jwt/jwt/v5"
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
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestRegisterUserServiceHTTPRoutesRegistersCurrentRouteGraph(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(openAPIEnabledEnv, "true")

	engine := gin.New()
	authorizer := &routerRegistrationAuthorizer{allowed: false}
	params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
		metrics:    metricsRouteConfig(true, "/internal/metrics"),
		authorizer: authorizer,
	})

	require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

	routes := collectRouterRegistrationRoutes(engine)
	requireRouterRoutesContain(t, routes, append(routerRegistrationRuntimeRoutes(), routerRegistrationV1Routes()...))
	requireRouterRoutesAbsent(t, routes, []routerRegisteredRoute{
		{method: http.MethodGet, path: "/metrics"},
		{method: http.MethodGet, path: "/debug/pprof"},
		{method: http.MethodGet, path: "/debug/pprof/*profile"},
		{method: http.MethodGet, path: "/internal/debug/pprof"},
		{method: http.MethodGet, path: "/internal/debug/pprof/*profile"},
		{method: http.MethodPost, path: "/api/auth/login"},
		{method: http.MethodPost, path: "/v1/auth/login"},
		{method: http.MethodGet, path: "/api/users"},
		{method: http.MethodGet, path: "/v1/users"},
	})

	publicAuth := executeRouterRegistrationRequest(engine, http.MethodPost, "/api/v1/auth/login", "", "")
	require.Equal(t, http.StatusBadRequest, publicAuth.Code, "body=%s", publicAuth.Body.String())

	protectedAuth := executeRouterRegistrationRequest(engine, http.MethodPost, "/api/v1/auth/logout", "", "")
	require.Equal(t, http.StatusUnauthorized, protectedAuth.Code, "body=%s", protectedAuth.Body.String())

	accessToken := signRouterRegistrationAccessToken(t)
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

func TestAuthorizedRouteGraphMatchesPermissionBaseline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{})
	require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

	actual := make(map[string]struct{})
	for _, route := range engine.Routes() {
		if route.Method == http.MethodOptions || !strings.HasPrefix(route.Path, "/api/v1/") || strings.HasPrefix(route.Path, "/api/v1/auth/") {
			continue
		}
		actual[route.Method+" "+route.Path] = struct{}{}
	}
	expected := make(map[string]struct{})
	for _, permission := range rbacbaseline.DefaultPermissions() {
		expected[permission.Method+" "+permission.PathTemplate] = struct{}{}
	}
	require.Equal(t, expected, actual)
}

func TestRegisterUserServiceHTTPRoutesRejectsMissingSecurityDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		remove  func(*RouteParams)
		wantErr string
	}{
		{name: "token version validator", remove: func(params *RouteParams) { params.TokenVersionValidator = nil }, wantErr: "token version validator is required"},
		{name: "authorizer", remove: func(params *RouteParams) { params.Authorizer = nil }, wantErr: "rbac authorizer is required"},
		{name: "auth controller", remove: func(params *RouteParams) { params.Auth = nil }, wantErr: "auth controller is required"},
		{name: "permission controller", remove: func(params *RouteParams) { params.Permission = nil }, wantErr: "permission controller is required"},
		{name: "role controller", remove: func(params *RouteParams) { params.Role = nil }, wantErr: "role controller is required"},
		{name: "user controller", remove: func(params *RouteParams) { params.User = nil }, wantErr: "user controller is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{})
			tt.remove(&params)

			err := RegisterUserServiceHTTPRoutes(engine, params)
			require.EqualError(t, err, tt.wantErr)
			require.Empty(t, engine.Routes())
		})
	}
}

func TestRegisterUserServiceHTTPRoutesReturnsMetricsConfigError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(openAPIEnabledEnv, "true")

	engine := gin.New()
	params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
		metrics: metricsRouteConfig(true, "/api/v1/metrics"),
	})

	err := RegisterUserServiceHTTPRoutes(engine, params)
	require.ErrorIs(t, err, ErrInvalidMetricsPath)
}

type routerRegisteredRoute struct {
	method string
	path   string
}

type routerRegistrationRouteOptions struct {
	metrics    config.MetricsConfig
	authorizer permissionauthorization.Authorizer
}

type routerRegistrationAuthorizer struct {
	allowed bool
	calls   int
}

type routerRegistrationTokenVersionValidator struct{}

func (routerRegistrationTokenVersionValidator) ValidateTokenVersion(context.Context, string, int64) error {
	return nil
}

func (a *routerRegistrationAuthorizer) Enforce(context.Context, string, string, string) (bool, error) {
	a.calls++
	return a.allowed, nil
}

func newRouterRegistrationRouteParams(t *testing.T, opts routerRegistrationRouteOptions) RouteParams {
	t.Helper()
	validator := newRouterRegistrationValidator(t)
	metricsCfg := opts.metrics
	if metricsCfg.Path == "" {
		metricsCfg = metricsRouteConfig(false, "/metrics")
	}
	authorizer := opts.authorizer
	if authorizer == nil {
		authorizer = &routerRegistrationAuthorizer{allowed: true}
	}
	return RouteParams{
		ServiceName:           "aegiscore-user-service-test",
		Environment:           "test",
		Log:                   zap.NewNop(),
		JWT:                   routerRegistrationAccessVerifier{},
		MetricsConfig:         metricsCfg,
		Metrics:               newRouterTestMetricsProvider(t, metricsCfg.Enabled, metricsCfg.Path),
		TokenVersionValidator: routerRegistrationTokenVersionValidator{},
		Authorizer:            authorizer,
		Auth:                  newRouterRegistrationAuthController(validator),
		Permission:            permissionhttp.NewPermissionController(nil, validator),
		Role:                  rolehttp.NewRoleController(nil, nil, validator),
		User:                  userhttp.NewUserController(nil, nil, validator),
	}
}

func newRouterRegistrationAuthController(validator *commonvalidation.Validator) *authhttp.AuthController {
	return authhttp.NewAuthController(authhttp.AuthControllerOptions{Validator: validator})
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

func signRouterRegistrationAccessToken(t *testing.T) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, routerRegistrationClaims{UserID: uuid.NewString(), TokenVersion: 1, SessionID: uuid.NewString(), RegisteredClaims: jwtv5.RegisteredClaims{Subject: "access", ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Minute))}}).SignedString([]byte("router-registration-secret"))
	require.NoError(t, err)
	return token
}

type routerRegistrationClaims struct {
	UserID       string `json:"user_id"`
	TokenVersion int64  `json:"token_version"`
	SessionID    string `json:"session_id"`
	jwtv5.RegisteredClaims
}

type routerRegistrationAccessVerifier struct{}

func (routerRegistrationAccessVerifier) VerifyAccessToken(token string) (commonauth.AccessToken, error) {
	claims := &routerRegistrationClaims{}
	parsed, err := jwtv5.ParseWithClaims(token, claims, func(*jwtv5.Token) (any, error) { return []byte("router-registration-secret"), nil }, jwtv5.WithExpirationRequired())
	if err != nil {
		return commonauth.AccessToken{}, err
	}
	if parsed == nil || !parsed.Valid || claims.Subject != "access" {
		return commonauth.AccessToken{}, jwtv5.ErrTokenInvalidClaims
	}
	return commonauth.AccessToken{UserID: claims.UserID, SessionID: claims.SessionID, TokenVersion: claims.TokenVersion}, nil
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
	routes = append(routes, routerRegistrationPermissionRouteList()...)
	routes = append(routes, routerRegistrationRoleRouteList()...)
	return routes
}

func routerRegistrationPermissionRouteList() []routerRegisteredRoute {
	return []routerRegisteredRoute{
		{method: http.MethodGet, path: "/api/v1/permissions"},
		{method: http.MethodGet, path: "/api/v1/permissions/users/:user_id/effective"},
	}
}

func routerRegistrationRoleRouteList() []routerRegisteredRoute {
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
