package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

const routeAuthUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
const routeAuthForbiddenUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3403"
const routeAuthNotFoundUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3999"
const routeAuthInternalErrorUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3500"

func TestGinEngineAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OPENAPI_ENABLED", "true")

	cfg := &config.Config{
		App: config.AppConfig{Name: "configured-user-service", Environment: "local"},
		Observability: config.ObservabilityConfig{
			Metrics: config.MetricsConfig{Enabled: true, Path: "/metrics", IncludeRuntime: true},
			Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, OTLPEndpoint: "127.0.0.1:4317", Insecure: true},
		},
	}
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)
	serviceCfg := &serviceconfig.Config{Auth: serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{Secret: "secret"}}}
	jwtService := authtokens.NewAccessTokenVerifier(commonauth.NewJWTService(commonauth.JWTConfig{Secret: serviceCfg.Auth.JWT.Secret}), serviceCfg)
	tokenVersions := &routeTokenVersionValidator{version: 1}
	authorizer := &routeAuthorizer{allowed: true}
	metricsProvider := newRouteTestMetricsProvider(t, cfg)
	traceProvider := newRouteTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Log: log, Metrics: metricsProvider, Trace: traceProvider})
	require.NoError(t, err)
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	err = RegisterRoutes(RegisterRouteParams{
		Config:        cfg,
		Log:           log,
		Engine:        engine,
		JWT:           jwtService,
		TokenVersions: tokenVersions,
		Authorizer:    authorizer,
		Metrics:       metricsProvider,
		AuthController: authhttp.NewAuthController(authhttp.AuthControllerParams{
			Login:          &routeAuthAuthUseCases{},
			Refresh:        &routeAuthAuthUseCases{},
			ChangePassword: &routeAuthAuthUseCases{},
			LogoutCurrent:  &routeAuthAuthUseCases{},
			LogoutAll:      &routeAuthAuthUseCases{},
			Validator:      validator,
		}),
		PermissionController: permissionhttp.NewPermissionController(nil, nil, validator),
		RoleController:       rolehttp.NewRoleController(nil, nil, validator),
		UserController:       userhttp.NewUserController(&routeAuthUserCommands{}, &routeAuthUserQueries{}, validator),
	})
	require.NoError(t, err)
	registerRouteTestRuntimeMetrics(t, cfg, metricsProvider)

	publicRequests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/livez"},
		{method: http.MethodGet, path: "/readyz"},
		{method: http.MethodGet, path: "/startupz"},
		{method: http.MethodGet, path: "/openapi/index.html"},
		{method: http.MethodGet, path: "/openapi.json"},
		{method: http.MethodGet, path: "/docs"},
		{method: http.MethodGet, path: "/api-docs"},
		{method: http.MethodGet, path: "/metrics"},
		{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"username":"alice","password":"secret"}`},
		{method: http.MethodPost, path: "/api/v1/auth/refresh", body: `{"refresh_token":"refresh"}`},
		{method: http.MethodPost, path: "/api/v1/auth/change-password", body: `{"new_password":"NewPassword123!"}`},
	}
	for _, tt := range publicRequests {
		t.Run("public "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			if tt.path == "/api/v1/auth/change-password" {
				request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+"password-change-token")
			}
			engine.ServeHTTP(recorder, request)
			require.NotEqual(t, http.StatusUnauthorized, recorder.Code)
		})
	}
	require.Equal(t, 0, authorizer.calls)

	t.Run("metrics route returns prometheus text without auth or rbac", func(t *testing.T) {
		initialLogCount := logs.Len()
		authorizer.reset()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
		require.Contains(t, recorder.Header().Get("Content-Type"), "text/plain")
		require.Contains(t, recorder.Body.String(), "go_goroutines")
		body := recorder.Body.String()
		for _, family := range []string{
			"aegiscore_postgres_pool_open_connections",
			"aegiscore_redis_up",
			"aegiscore_workerpool_tasks_total",
			"aegiscore_localcache_requests_total",
			"aegiscore_localcache_loads_total",
			"aegiscore_localcache_singleflight_total",
			"aegiscore_localcache_writes_total",
			"aegiscore_localcache_evictions_total",
			"aegiscore_localcache_capacity",
			"aegiscore_runtime_component_running",
		} {
			assert.Contains(t, body, family)
		}
		require.Equal(t, 0, authorizer.calls)
		require.Equal(t, initialLogCount, logs.Len())
	})

	t.Run("probes return configured service name", func(t *testing.T) {
		initialLogCount := logs.Len()
		for _, path := range []string{"/livez", "/readyz", "/startupz"} {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			engine.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code)
			var health struct {
				Status  string `json:"status"`
				Service string `json:"service"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &health))
			require.Equal(t, "ok", health.Status)
			require.Equal(t, cfg.App.Name, health.Service)
		}
		require.Equal(t, initialLogCount, logs.Len())
	})

	t.Run("query requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, contracterrors.CodeUnauthenticated)
	})

	t.Run("create requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{"nickname":"Alice","username":"alice"}`))
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, contracterrors.CodeUnauthenticated)
	})

	t.Run("list requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, contracterrors.CodeUnauthenticated)
	})

	t.Run("logout requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, contracterrors.CodeUnauthenticated)
	})

	t.Run("logout all requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, contracterrors.CodeUnauthenticated)
	})
	if authorizer.calls != 0 {
		require.Equal(t, 0, authorizer.calls)
	}

	t.Run("query with invalid token returns token invalid", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+"invalid")
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, contracterrors.CodeTokenInvalid)
	})

	t.Run("query with valid token keeps controller behavior", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		authorizer.reset()
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
		assertSuccessEnvelope(t, recorder)
		require.Equal(t, 1, authorizer.calls)
		require.Equal(t, routeAuthUserID, authorizer.userID)
		require.Equal(t, "/api/v1/users/:user_id", authorizer.pathTemplate)
		require.Equal(t, http.MethodGet, authorizer.method)

		entries := logs.FilterMessage("http request completed").All()
		require.NotEmpty(t, entries)
		fields := entries[len(entries)-1].ContextMap()
		assert.True(t, validTraceIDField(fields[logger.TraceIDField]), "fields=%#v", fields)
		assert.True(t, validSpanIDField(fields[logger.SpanIDField]), "fields=%#v", fields)
		assert.Equal(t, http.MethodGet, fields["method"])
		assert.Equal(t, "/api/v1/users/:user_id", fields["path"])
		assert.Equal(t, int64(http.StatusOK), fields["status"])
		assert.Equal(t, routeAuthUserID, fields[commonauth.UserIDKey])
		assert.Contains(t, fields, "latency_ms")
		assert.Contains(t, fields, "client_ip")
	})

	t.Run("query denied by rbac returns forbidden before controller", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		authorizer.reset()
		authorizer.allowed = false
		defer func() { authorizer.allowed = true }()
		engine.ServeHTTP(recorder, request)
		assertFailureEnvelope(t, recorder, http.StatusForbidden, contracterrors.CodeForbidden)
		require.Equal(t, 1, authorizer.calls)
	})

	t.Run("query with mismatched token version returns token invalid", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthTokenWithVersion(t, "secret", routeAuthUserID, 2))
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, contracterrors.CodeTokenInvalid)
	})

	t.Run("query not found returns route envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthNotFoundUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		assertFailureEnvelope(t, recorder, http.StatusNotFound, contracterrors.CodeNotFound)
	})

	t.Run("query forbidden returns route envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthForbiddenUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		assertFailureEnvelope(t, recorder, http.StatusForbidden, contracterrors.CodeForbidden)
	})

	t.Run("query internal error returns route envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthInternalErrorUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		assertFailureEnvelope(t, recorder, http.StatusInternalServerError, contracterrors.CodeInternalError)
	})

	t.Run("panic recovery returns envelope and logs trace and span ids", func(t *testing.T) {
		engine.GET("/panic-route-chain", func(_ *gin.Context) { panic("route-chain boom") })
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/panic-route-chain", nil)
		engine.ServeHTTP(recorder, request)

		assertFailureEnvelope(t, recorder, http.StatusInternalServerError, contracterrors.CodeInternalError)
		entries := logs.FilterMessage("panic recovered").All()
		require.Len(t, entries, 1)
		fields := entries[0].ContextMap()
		assert.True(t, validTraceIDField(fields[logger.TraceIDField]), "fields=%#v", fields)
		assert.True(t, validSpanIDField(fields[logger.SpanIDField]), "fields=%#v", fields)
		assert.Equal(t, "route-chain boom", fields["panic"])
		assert.Contains(t, fields, "stack")
	})

	t.Run("query validation still runs after auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}
