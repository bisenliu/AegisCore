package providers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/logger"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/common/runtime/workerpool"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"github.com/aegiscore/user-service/internal/shared/identity"
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
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{Secret: "secret"},
		},
		Observability: config.ObservabilityConfig{
			Metrics: config.MetricsConfig{Enabled: true, Path: "/metrics", IncludeRuntime: true},
			Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, Exporter: "none"},
		},
	}
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)
	jwtService := commonauth.NewJWTService(cfg.Auth)
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
		UserController: userhttp.NewUserController(&routeAuthUserCommands{}, &routeAuthUserQueries{}, validator),
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

func TestRegisterRoutesRejectsMetricsPathConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		App: config.AppConfig{Name: "configured-user-service", Environment: "local"},
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{Secret: "secret"},
		},
		Observability: config.ObservabilityConfig{
			Metrics: config.MetricsConfig{Enabled: true, Path: "/api/v1/metrics"},
		},
	}
	validator := mustRouteTestValidator(t)
	err := RegisterRoutes(RegisterRouteParams{
		Config:     cfg,
		Log:        zap.NewNop(),
		Engine:     gin.New(),
		JWT:        commonauth.NewJWTService(cfg.Auth),
		Authorizer: &routeAuthorizer{allowed: true},
		Metrics:    newRouteTestMetricsProvider(t, cfg),
		AuthController: authhttp.NewAuthController(authhttp.AuthControllerParams{
			Login:          &routeAuthAuthUseCases{},
			Refresh:        &routeAuthAuthUseCases{},
			ChangePassword: &routeAuthAuthUseCases{},
			LogoutCurrent:  &routeAuthAuthUseCases{},
			LogoutAll:      &routeAuthAuthUseCases{},
			Validator:      validator,
		}),
		UserController: userhttp.NewUserController(&routeAuthUserCommands{}, &routeAuthUserQueries{}, validator),
	})
	require.ErrorContains(t, err, "invalid metrics path")
}

func newRouteTestTracingProvider(t *testing.T, cfg *config.Config) *commontracing.Provider {
	t.Helper()
	provider, err := commontracing.NewProvider(context.Background(), commontracing.Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return provider
}

func newRouteTestMetricsProvider(t *testing.T, cfg *config.Config) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      cfg.Observability.Metrics,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	require.NoError(t, err)
	return provider
}

func registerRouteTestRuntimeMetrics(t *testing.T, cfg *config.Config, provider *commonmetrics.Provider) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:route_runtime_metrics?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	if cfg.Redis == nil {
		cfg.Redis = make(map[string]config.RedisConfig)
	}
	cfg.Redis[resources.NameCacheRedis] = config.RedisConfig{PingTimeout: time.Second}

	err = RegisterRuntimeDependencyMetrics(RuntimeDependencyMetricsParams{
		Config:           cfg,
		Metrics:          provider,
		UserDB:           db,
		CacheRedis:       client,
		SessionPurgePool: routePurgeTaskPool{stats: workerpool.Stats{Name: "auth.redis.session_purge", Workers: 4, Submitted: 1}},
		PolicyWatcher:    stubWatcherStatus{running: true},
		AuthTokenCache:   fakeLocalcacheStatsSource{name: "auth_token_version", stats: localcache.Stats{Capacity: 1000}},
		RBACRolesCache:   fakeLocalcacheStatsSource{name: "rbac_user_roles", stats: localcache.Stats{Capacity: 2000}},
	})
	require.NoError(t, err)
}

func mustRouteTestValidator(t *testing.T) *validation.Validator {
	t.Helper()
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	return validator
}

func validTraceIDField(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	traceID, err := oteltrace.TraceIDFromHex(text)
	return err == nil && traceID.IsValid()
}

func validSpanIDField(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	spanID, err := oteltrace.SpanIDFromHex(text)
	return err == nil && spanID.IsValid()
}

type routeAuthUserCommands struct{}

type routeAuthUserQueries struct{}

type routeAuthAuthUseCases struct{}

type routePurgeTaskPool struct {
	stats workerpool.Stats
}

func (p routePurgeTaskPool) Submit(context.Context, workerpool.Task) error {
	return nil
}

func (p routePurgeTaskPool) Stats() workerpool.Stats {
	return p.stats
}

type routeTokenVersionValidator struct {
	version int64
}

var _ permissionauthorization.Authorizer = (*routeAuthorizer)(nil)

type routeAuthorizer struct {
	allowed      bool
	err          error
	calls        int
	userID       string
	pathTemplate string
	method       string
}

func (a *routeAuthorizer) Enforce(_ context.Context, userID string, pathTemplate string, method string) (bool, error) {
	a.calls++
	a.userID = userID
	a.pathTemplate = pathTemplate
	a.method = method
	return a.allowed, a.err
}

func (a *routeAuthorizer) reset() {
	a.calls = 0
	a.userID = ""
	a.pathTemplate = ""
	a.method = ""
	a.err = nil
}

func (s *routeTokenVersionValidator) ValidateTokenVersion(_ context.Context, _ string, tokenVersion int64) error {
	if tokenVersion != s.version {
		return &commonauth.TokenVersionMismatchError{Current: s.version, Token: tokenVersion}
	}
	return nil
}

func (s *routeAuthAuthUseCases) Login(context.Context, authcommand.LoginCommand) (*authtokens.TokenResult, error) {
	return &authtokens.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthUseCases) Refresh(context.Context, authcommand.RefreshTokenCommand) (*authtokens.TokenResult, error) {
	return &authtokens.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthUseCases) ChangePassword(context.Context, authcommand.ChangePasswordCommand) (*authcommand.ChangePasswordResult, error) {
	return &authcommand.ChangePasswordResult{Changed: true}, nil
}

func (s *routeAuthAuthUseCases) LogoutCurrentSession(context.Context) (*authcommand.LogoutResult, error) {
	return &authcommand.LogoutResult{LoggedOut: true}, nil
}

func (s *routeAuthAuthUseCases) LogoutAllSessions(context.Context) (*authcommand.LogoutResult, error) {
	return &authcommand.LogoutResult{LoggedOut: true}, nil
}

func (s *routeAuthUserCommands) CreateUser(context.Context, usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
	now := time.Now().UnixMilli()
	return &usercommand.CreateUserResult{User: userdomain.User{UserID: uuid.MustParse(routeAuthUserID), Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}, nil
}

func (s *routeAuthUserQueries) GetUserByID(_ context.Context, req userquery.GetUserByIDQuery) (*userquery.GetUserResult, error) {
	userID := req.UserID
	userIDString := userID.String()
	if userIDString == routeAuthNotFoundUserID {
		return nil, identity.ErrUserNotFound
	}
	if userIDString == routeAuthForbiddenUserID {
		return nil, contracterrors.ForbiddenError("forbidden")
	}
	if userIDString == routeAuthInternalErrorUserID {
		return nil, errors.New("database down")
	}
	now := time.Now().UnixMilli()
	return &userquery.GetUserResult{User: userdomain.User{UserID: userID, Nickname: "Aegis", Username: "aegis", Status: identity.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}, nil
}

func assertSuccessEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.Equal(t, contracterrors.CodeOK, envelope.Code)
}

func assertFailureEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode contracterrors.Code) {
	t.Helper()
	require.Equal(t, wantStatus, recorder.Code, "body=%s", recorder.Body.String())
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.False(t, envelope.Success)
	require.Equal(t, wantCode, envelope.Code)
}

func (s *routeAuthUserQueries) ListUsers(context.Context, userquery.ListUsersQuery) (*userquery.ListUsersResult, error) {
	now := time.Now().UnixMilli()
	items := []userdomain.User{{UserID: uuid.MustParse(routeAuthUserID), Nickname: "Aegis", Username: "aegis", Status: identity.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}
	return &userquery.ListUsersResult{Items: items, PageSize: 10}, nil
}

func assertAuthFailureEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantCode contracterrors.Code) {
	t.Helper()
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.False(t, envelope.Success)
	require.Equal(t, wantCode, envelope.Code)
}

func signRouteAuthToken(t *testing.T, secret, userID string) string {
	return signRouteAuthTokenWithVersion(t, secret, userID, 1)
}

func signRouteAuthTokenWithVersion(t *testing.T, secret, userID string, tokenVersion int64) string {
	t.Helper()
	_, err := uuid.Parse(userID)
	require.NoError(t, err)
	tokenID, err := runtimeid.NewUUID()
	require.NoError(t, err)
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, commonauth.Claims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        "s-123",
		RegisteredClaims: jwtv5.RegisteredClaims{ID: tokenID.String(), Subject: commonauth.SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}
