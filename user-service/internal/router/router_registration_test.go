package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonmw "github.com/aegiscore/common/http/middleware"
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

func TestRegisterUserServiceHTTPRoutesAppliesRateLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("public auth rate limit rejects before controller", func(t *testing.T) {
		engine := gin.New()
		params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{anonymousLimiter: &routerRegistrationLimiter{allowed: false}})
		require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

		recorder := executeRouterRegistrationRequest(engine, http.MethodPost, "/api/v1/auth/login", "", "{}")

		require.Equal(t, http.StatusTooManyRequests, recorder.Code, "body=%s", recorder.Body.String())
		requireRouterRegistrationFailureCode(t, recorder, contracterrors.CodeRateLimited)
	})

	t.Run("authentication failure does not consume user limiter", func(t *testing.T) {
		userLimiter := &routerRegistrationLimiter{allowed: false}
		engine := gin.New()
		params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{userLimiter: userLimiter})
		require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

		recorder := executeRouterRegistrationRequest(engine, http.MethodGet, "/api/v1/users", "", "")

		require.Equal(t, http.StatusUnauthorized, recorder.Code, "body=%s", recorder.Body.String())
		require.Equal(t, 0, userLimiter.calls)
	})

	t.Run("authenticated route rate limit rejects before rbac", func(t *testing.T) {
		authorizer := &routerRegistrationAuthorizer{allowed: true}
		engine := gin.New()
		params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{authorizer: authorizer, userLimiter: &routerRegistrationLimiter{allowed: false}})
		require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

		recorder := executeRouterRegistrationRequest(engine, http.MethodGet, "/api/v1/users", signRouterRegistrationAccessToken(t), "")

		require.Equal(t, http.StatusTooManyRequests, recorder.Code, "body=%s", recorder.Body.String())
		requireRouterRegistrationFailureCode(t, recorder, contracterrors.CodeRateLimited)
		require.Equal(t, 0, authorizer.calls)
	})

	t.Run("authenticated route continues to rbac when allowed", func(t *testing.T) {
		authorizer := &routerRegistrationAuthorizer{allowed: false}
		engine := gin.New()
		params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{authorizer: authorizer, userLimiter: &routerRegistrationLimiter{allowed: true}})
		require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

		recorder := executeRouterRegistrationRequest(engine, http.MethodGet, "/api/v1/users", signRouterRegistrationAccessToken(t), "")

		require.Equal(t, http.StatusForbidden, recorder.Code, "body=%s", recorder.Body.String())
		require.Equal(t, 1, authorizer.calls)
	})

	t.Run("runtime routes bypass business rate limiters", func(t *testing.T) {
		anonymousLimiter := &routerRegistrationLimiter{allowed: false}
		userLimiter := &routerRegistrationLimiter{allowed: false}
		engine := gin.New()
		params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{anonymousLimiter: anonymousLimiter, userLimiter: userLimiter})
		require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

		recorder := executeRouterRegistrationRequest(engine, http.MethodGet, "/livez", "", "")

		require.NotEqual(t, http.StatusTooManyRequests, recorder.Code, "body=%s", recorder.Body.String())
		require.Equal(t, 0, anonymousLimiter.calls)
		require.Equal(t, 0, userLimiter.calls)
	})
}

func TestRegisterUserServiceHTTPRoutesRecordsRateLimitObservability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.DebugLevel)
	metricsCfg := metricsRouteConfig(true, "/internal/metrics")
	params := newRouterRegistrationRouteParams(t, routerRegistrationRouteOptions{
		metrics:          metricsCfg,
		anonymousLimiter: &routerRegistrationLimiter{err: commonmw.ErrRateLimitKeyRequired},
		userLimiter:      &routerRegistrationLimiter{allowed: false},
	})
	params.Log = zap.New(core)
	engine := gin.New()
	require.NoError(t, RegisterUserServiceHTTPRoutes(engine, params))

	publicAuth := executeRouterRegistrationRequest(engine, http.MethodPost, "/api/v1/auth/login", "", "{}")
	require.NotEqual(t, http.StatusTooManyRequests, publicAuth.Code, "body=%s", publicAuth.Body.String())

	protected := executeRouterRegistrationRequest(engine, http.MethodGet, "/api/v1/users", signRouterRegistrationAccessToken(t), "")
	require.Equal(t, http.StatusTooManyRequests, protected.Code, "body=%s", protected.Body.String())

	families, err := params.Metrics.GatherContext(context.Background())
	require.NoError(t, err)
	requireMetricCounterValue(t, families, apiRateLimitEventsMetricName, map[string]string{"scope": "anonymous_auth", "event": "error", "reason": "key_required"}, 1)
	requireMetricCounterValue(t, families, apiRateLimitEventsMetricName, map[string]string{"scope": "authenticated_api", "event": "limited", "reason": "limit_exceeded"}, 1)
	require.Len(t, logs.FilterMessage("api rate limiter failed").All(), 1)
	require.Len(t, logs.FilterMessage("api request rate limited").All(), 1)
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
	metrics          config.MetricsConfig
	authorizer       permissionauthorization.Authorizer
	anonymousLimiter commonmw.RateLimiter
	userLimiter      commonmw.RateLimiter
}

type routerRegistrationAuthorizer struct {
	allowed bool
	calls   int
}

type routerRegistrationTokenVersionValidator struct{}

type routerRegistrationLimiter struct {
	allowed bool
	err     error
	calls   int
	keys    []string
}

func (routerRegistrationTokenVersionValidator) ValidateTokenVersion(context.Context, string, int64) error {
	return nil
}

func (a *routerRegistrationAuthorizer) Enforce(context.Context, string, string, string) (bool, error) {
	a.calls++
	return a.allowed, nil
}

func (l *routerRegistrationLimiter) Allow(key string) (bool, error) {
	l.calls++
	l.keys = append(l.keys, key)
	if l.err != nil {
		return false, l.err
	}
	return l.allowed, nil
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
		AnonymousRateLimiter:  opts.anonymousLimiter,
		UserRateLimiter:       opts.userLimiter,
		TokenVersionValidator: routerRegistrationTokenVersionValidator{},
		Authorizer:            authorizer,
		Auth:                  newRouterRegistrationAuthController(validator),
		Permission:            permissionhttp.NewPermissionController(nil, validator),
		Role:                  rolehttp.NewRoleController(nil, nil, validator),
		User:                  userhttp.NewUserController(nil, nil, validator),
	}
}

func requireRouterRegistrationFailureCode(t *testing.T, recorder *httptest.ResponseRecorder, wantCode contracterrors.Code) {
	t.Helper()
	var envelope struct {
		Success bool                `json:"success"`
		Code    contracterrors.Code `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.False(t, envelope.Success)
	require.Equal(t, wantCode, envelope.Code)
}

func requireMetricCounterValue(t *testing.T, families []*io_prometheus_client.MetricFamily, name string, labels map[string]string, want float64) {
	t.Helper()
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metricLabelsMatch(metric, labels) {
				require.NotNil(t, metric.GetCounter(), "metric %s is not a counter", name)
				require.Equal(t, want, metric.GetCounter().GetValue())
				return
			}
		}
	}
	require.Failf(t, "missing metric", "name=%s labels=%v", name, labels)
}

func metricLabelsMatch(metric *io_prometheus_client.Metric, want map[string]string) bool {
	got := make(map[string]string, len(metric.GetLabel()))
	for _, label := range metric.GetLabel() {
		got[label.GetName()] = label.GetValue()
	}
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
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
