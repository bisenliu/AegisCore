package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	authapp "github.com/aegiscore/user-services/internal/features/auth/app"
	authhttp "github.com/aegiscore/user-services/internal/features/auth/transport/http"
	userapp "github.com/aegiscore/user-services/internal/features/user/app"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	userhttp "github.com/aegiscore/user-services/internal/features/user/transport/http"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const routeAuthUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
const routeAuthForbiddenUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3403"
const routeAuthNotFoundUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3999"
const routeAuthInternalErrorUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3500"

type lifecycleRecorder struct {
	hooks []fx.Hook
}

func (r *lifecycleRecorder) Append(hook fx.Hook) {
	r.hooks = append(r.hooks, hook)
}

type shutdownRecorder struct {
	calls int
	err   error
}

func (r *shutdownRecorder) Shutdown(...fx.ShutdownOption) error {
	r.calls++
	return r.err
}

func TestDefaultConfigHTTPTimeouts(t *testing.T) {
	cfg, err := config.Load("../../configs/config.yaml")
	if err != nil {
		t.Fatalf("Load default config: %v", err)
	}

	if cfg.HTTP.ReadTimeout != 30*time.Second || cfg.HTTP.WriteTimeout != 60*time.Second || cfg.HTTP.IdleTimeout != 120*time.Second || cfg.HTTP.ShutdownTimeout != 25*time.Second {
		t.Fatalf("HTTP timeouts = (%s,%s,%s,%s), want (30s,60s,120s,25s)", cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout, cfg.HTTP.IdleTimeout, cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Auth.JWT.Secret == "" || cfg.Auth.JWT.Issuer != "aegiscore-user-services" || cfg.Auth.JWT.Audience != "aegiscore-users" {
		t.Fatalf("Auth.JWT = %#v, want default auth config", cfg.Auth.JWT)
	}
	if cfg.Auth.JWT.AccessTokenTTL != 15*time.Minute || cfg.Auth.JWT.RefreshTokenTTL != 168*time.Hour || cfg.Auth.TokenVersionCacheTTL != 5*time.Minute {
		t.Fatalf("auth TTLs = (%s,%s,%s), want (15m,168h,5m)", cfg.Auth.JWT.AccessTokenTTL, cfg.Auth.JWT.RefreshTokenTTL, cfg.Auth.TokenVersionCacheTTL)
	}
}

func TestHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            18080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    60 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 25 * time.Second,
		},
	}
	server := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config:    cfg,
		Log:       zap.NewNop(),
		Engine:    gin.New(),
	})

	if server.ReadTimeout != cfg.HTTP.ReadTimeout || server.WriteTimeout != cfg.HTTP.WriteTimeout || server.IdleTimeout != cfg.HTTP.IdleTimeout {
		t.Fatalf("server timeouts = (%s,%s,%s), want (%s,%s,%s)", server.ReadTimeout, server.WriteTimeout, server.IdleTimeout, cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout, cfg.HTTP.IdleTimeout)
	}
	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStop == nil {
		t.Fatalf("lifecycle hooks = %#v, want one shutdown hook", lifecycle.hooks)
	}
}

func TestHTTPServerStartReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	lifecycle := &lifecycleRecorder{}
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{HTTP: config.HTTPConfig{
			Host: "127.0.0.1",
			Port: addr.Port,
		}},
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})

	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStart == nil {
		t.Fatalf("lifecycle hooks = %#v, want one start hook", lifecycle.hooks)
	}
	err = lifecycle.hooks[0].OnStart(context.Background())
	if err == nil {
		t.Fatal("OnStart error = nil, want listen error")
	}
	if !strings.Contains(err.Error(), "listen http server") {
		t.Fatalf("OnStart error = %q, want listen http server context", err.Error())
	}
}

func TestHTTPServerUnexpectedServeErrorTriggersShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner := &shutdownRecorder{}
	serveErr := errors.New("serve failed")

	shutdownOnHTTPServeError(zap.New(core), shutdowner, serveErr)

	if shutdowner.calls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdowner.calls)
	}
	entries := logs.FilterMessage("http server failed").All()
	if len(entries) != 1 {
		t.Fatalf("http server failed logs = %d, want 1", len(entries))
	}
	if loggedErr, ok := entries[0].ContextMap()["error"].(string); !ok || loggedErr != serveErr.Error() {
		t.Fatalf("logged error = %#v, want %q", entries[0].ContextMap()["error"], serveErr.Error())
	}
}

func TestHTTPServerUnexpectedServeErrorLogsShutdownFailure(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdownErr := errors.New("shutdown failed")
	shutdowner := &shutdownRecorder{err: shutdownErr}

	shutdownOnHTTPServeError(zap.New(core), shutdowner, errors.New("serve failed"))

	if shutdowner.calls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdowner.calls)
	}
	entries := logs.FilterMessage("shutdown after http server failure failed").All()
	if len(entries) != 1 {
		t.Fatalf("shutdown failure logs = %d, want 1", len(entries))
	}
	if loggedErr, ok := entries[0].ContextMap()["error"].(string); !ok || loggedErr != shutdownErr.Error() {
		t.Fatalf("logged shutdown error = %#v, want %q", entries[0].ContextMap()["error"], shutdownErr.Error())
	}
}

func TestHTTPServerClosedServeErrorDoesNotTriggerShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner := &shutdownRecorder{}

	shutdownOnHTTPServeError(zap.New(core), shutdowner, http.ErrServerClosed)
	shutdownOnHTTPServeError(zap.New(core), shutdowner, nil)

	if shutdowner.calls != 0 {
		t.Fatalf("shutdown calls = %d, want 0", shutdowner.calls)
	}
	if logs.Len() != 0 {
		t.Fatalf("error logs = %d, want 0", logs.Len())
	}
}

func TestHTTPServerLifecycleCancelStopsServeGoroutine(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	core, logs := observer.New(zapcore.DebugLevel)
	shutdowner := &shutdownRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		serveHTTPWithLifecycle(ctx, zap.New(core), shutdowner, &http.Server{}, listener)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serve goroutine did not exit after lifecycle context cancellation")
	}

	if shutdowner.calls != 0 {
		t.Fatalf("shutdown calls = %d, want 0", shutdowner.calls)
	}
	if entries := logs.FilterMessage("http server failed").All(); len(entries) != 0 {
		t.Fatalf("http server failed logs = %d, want 0", len(entries))
	}
	entries := logs.FilterMessage("http server goroutine stopped").All()
	if len(entries) != 1 {
		t.Fatalf("http server goroutine stopped logs = %d, want 1", len(entries))
	}
	if reason := entries[0].ContextMap()["reason"]; reason != "lifecycle_canceled" {
		t.Fatalf("goroutine stop reason = %#v, want lifecycle_canceled", reason)
	}
}

func TestHTTPServerStartAndStop(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            0,
			ShutdownTimeout: time.Second,
		}},
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})

	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStart == nil || lifecycle.hooks[0].OnStop == nil {
		t.Fatalf("lifecycle hooks = %#v, want one start/stop hook", lifecycle.hooks)
	}
	if err := lifecycle.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := lifecycle.hooks[0].OnStop(stopCtx); err != nil {
		t.Fatalf("OnStop: %v", err)
	}
}

func TestHTTPServerStartLogIncludesRuntimeIdentity(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{
			App:    config.AppConfig{Name: "aegiscore-user-services", Environment: "local"},
			System: config.SystemConfig{Timezone: "Asia/Shanghai"},
			HTTP: config.HTTPConfig{
				Host: "127.0.0.1",
				Port: 0,
			},
		},
		Log:    log,
		Engine: gin.New(),
	})

	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStart == nil || lifecycle.hooks[0].OnStop == nil {
		t.Fatalf("lifecycle hooks = %#v, want one start/stop hook", lifecycle.hooks)
	}
	if err := lifecycle.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := lifecycle.hooks[0].OnStop(stopCtx); err != nil {
		t.Fatalf("OnStop: %v", err)
	}

	entries := logs.FilterMessage("starting http server").All()
	if len(entries) != 1 {
		t.Fatalf("starting http server logs = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["addr"] != "127.0.0.1:0" || fields["service"] != "aegiscore-user-services" || fields["environment"] != "local" || fields["timezone"] != "Asia/Shanghai" {
		t.Fatalf("startup log fields = %#v", fields)
	}
}

func TestDefaultHTTPShutdownTimeout(t *testing.T) {
	if defaultHTTPShutdownTimeout != 10*time.Second {
		t.Fatalf("defaultHTTPShutdownTimeout = %s, want 10s", defaultHTTPShutdownTimeout)
	}
}

func TestGinEngineAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SWAGGER_ENABLED", "true")

	cfg := &config.Config{
		App: config.AppConfig{Name: "configured-user-service", Environment: "local"},
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{Secret: "secret"},
		},
	}
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)
	jwtService := commonauth.NewJWTService(cfg.Auth)
	tokenVersions := &routeTokenVersionValidator{version: 1}
	engine, err := NewGinEngine(GinParams{Config: cfg, Log: log})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	RegisterRoutes(RegisterRouteParams{
		Config:         cfg,
		Log:            log,
		Engine:         engine,
		JWT:            jwtService,
		TokenVersions:  tokenVersions,
		AuthController: authhttp.NewAuthController(&routeAuthAuthService{}, validator),
		UserController: userhttp.NewUserController(&routeAuthUserService{}, validator),
	})

	publicRequests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/healthz"},
		{method: http.MethodGet, path: "/swagger/index.html"},
		{method: http.MethodGet, path: "/docs"},
		{method: http.MethodGet, path: "/api-docs"},
		{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"username":"alice","password":"secret"}`},
		{method: http.MethodPost, path: "/api/v1/auth/refresh", body: `{"refresh_token":"refresh"}`},
		{method: http.MethodPost, path: "/api/v1/auth/change-password", body: `{"new_password":"NewPassword123!"}`},
	}
	for _, tt := range publicRequests {
		t.Run("public "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Trace-ID", "trace-public-test")
			if tt.path == "/api/v1/auth/change-password" {
				request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+"password-change-token")
			}
			engine.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusUnauthorized {
				t.Fatalf("%s status = %d, want not unauthorized", tt.path, recorder.Code)
			}
			if recorder.Header().Get("X-Trace-ID") != "trace-public-test" {
				t.Fatalf("X-Trace-ID = %q, want trace-public-test", recorder.Header().Get("X-Trace-ID"))
			}
		})
	}

	t.Run("healthz returns configured service name", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		var health struct {
			Status  string `json:"status"`
			Service string `json:"service"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &health); err != nil {
			t.Fatalf("unmarshal health response: %v", err)
		}
		if health.Status != "ok" || health.Service != cfg.App.Name {
			t.Fatalf("health = %#v, want configured service name", health)
		}
	})

	t.Run("query requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeUnauthenticated)
	})

	t.Run("create requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{"nickname":"Alice","username":"alice"}`))
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeUnauthenticated)
	})

	t.Run("list requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeUnauthenticated)
	})

	t.Run("logout requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeUnauthenticated)
	})

	t.Run("logout all requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", nil)
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeUnauthenticated)
	})

	t.Run("query with invalid token returns token invalid", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+"invalid")
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeTokenInvalid)
	})

	t.Run("query with valid token keeps controller behavior", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		request.Header.Set("X-Trace-ID", "trace-auth-test")
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if recorder.Header().Get("X-Trace-ID") != "trace-auth-test" {
			t.Fatalf("X-Trace-ID = %q, want trace-auth-test", recorder.Header().Get("X-Trace-ID"))
		}
		assertSuccessEnvelope(t, recorder)

		entries := logs.FilterMessage("http request completed").All()
		if len(entries) == 0 {
			t.Fatal("request log count = 0, want at least one")
		}
		fields := entries[len(entries)-1].ContextMap()
		if fields[logger.TraceIDField] != "trace-auth-test" || fields["method"] != http.MethodGet || fields["path"] != "/api/v1/users/"+routeAuthUserID || fields["status"] != int64(http.StatusOK) || fields[commonauth.UserIDKey] != routeAuthUserID {
			t.Fatalf("request log fields = %#v", fields)
		}
		if _, ok := fields["latency"]; !ok {
			t.Fatalf("request log missing latency: %#v", fields)
		}
		if _, ok := fields["client_ip"]; !ok {
			t.Fatalf("request log missing client_ip: %#v", fields)
		}
	})

	t.Run("query with mismatched token version returns token invalid", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthTokenWithVersion(t, "secret", routeAuthUserID, 2))
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeTokenInvalid)
	})

	t.Run("query not found returns route envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthNotFoundUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		assertFailureEnvelope(t, recorder, http.StatusNotFound, response.CodeNotFound)
	})

	t.Run("query forbidden returns route envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthForbiddenUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		assertFailureEnvelope(t, recorder, http.StatusForbidden, response.CodeForbidden)
	})

	t.Run("query internal error returns route envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthInternalErrorUserID, nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		assertFailureEnvelope(t, recorder, http.StatusInternalServerError, response.CodeInternalError)
	})

	t.Run("panic recovery returns envelope and logs trace id", func(t *testing.T) {
		engine.GET("/panic-route-chain", func(c *gin.Context) { panic("route-chain boom") })
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/panic-route-chain", nil)
		request.Header.Set("X-Trace-ID", "trace-panic-route-chain")
		engine.ServeHTTP(recorder, request)

		assertFailureEnvelope(t, recorder, http.StatusInternalServerError, response.CodeInternalError)
		entries := logs.FilterMessage("panic recovered").All()
		if len(entries) != 1 {
			t.Fatalf("panic recovered logs = %d, want 1", len(entries))
		}
		fields := entries[0].ContextMap()
		if fields[logger.TraceIDField] != "trace-panic-route-chain" || fields["panic"] != "route-chain boom" {
			t.Fatalf("recovery log fields = %#v", fields)
		}
		if _, ok := fields["stack"]; !ok {
			t.Fatalf("recovery log missing stack: %#v", fields)
		}
	})

	t.Run("query validation still runs after auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
		request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	})
}

type routeAuthUserService struct{}

type routeAuthAuthService struct{}

type routeTokenVersionValidator struct {
	version int64
}

func (s *routeTokenVersionValidator) ValidateTokenVersion(_ context.Context, _ string, tokenVersion int64) error {
	if tokenVersion != s.version {
		return &commonauth.TokenVersionMismatchError{Current: s.version, Token: tokenVersion}
	}
	return nil
}

func (s *routeAuthAuthService) Login(context.Context, authapp.LoginCommand) (*authapp.TokenResult, error) {
	return &authapp.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthService) Refresh(context.Context, authapp.RefreshTokenCommand) (*authapp.TokenResult, error) {
	return &authapp.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthService) ChangePassword(context.Context, authapp.ChangePasswordCommand) (*authapp.ChangePasswordResult, error) {
	return &authapp.ChangePasswordResult{Changed: true}, nil
}

func (s *routeAuthAuthService) Logout(context.Context) (*authapp.LogoutResult, error) {
	return &authapp.LogoutResult{LoggedOut: true}, nil
}

func (s *routeAuthAuthService) LogoutAll(context.Context) (*authapp.LogoutResult, error) {
	return &authapp.LogoutResult{LoggedOut: true}, nil
}

func (s *routeAuthUserService) CreateUser(context.Context, userapp.CreateUserCommand) (*userapp.UserResult, error) {
	now := time.Now().UnixMilli()
	return &userapp.UserResult{User: userdomain.User{UserID: uuid.MustParse(routeAuthUserID), Nickname: "Alice", Username: "alice", Status: userdomain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}, nil
}

func (s *routeAuthUserService) GetUserByID(_ context.Context, userID uuid.UUID) (*userapp.UserResult, error) {
	userIDString := userID.String()
	if userIDString == routeAuthNotFoundUserID {
		return nil, userdomain.ErrUserNotFound
	}
	if userIDString == routeAuthForbiddenUserID {
		return nil, response.ForbiddenError("forbidden")
	}
	if userIDString == routeAuthInternalErrorUserID {
		return nil, errors.New("database down")
	}
	now := time.Now().UnixMilli()
	return &userapp.UserResult{User: userdomain.User{UserID: userID, Nickname: "Aegis", Username: "aegis", Status: userdomain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}, nil
}

func assertSuccessEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("envelope = %#v, want success OK", envelope)
	}
}

func assertFailureEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode response.Code) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != wantCode {
		t.Fatalf("envelope = %#v, want failure code %d", envelope, wantCode)
	}
}

func (s *routeAuthUserService) ListUsers(context.Context, userapp.ListUsersQuery) (*userapp.ListUsersResult, error) {
	now := time.Now().UnixMilli()
	items := []userdomain.User{{UserID: uuid.MustParse(routeAuthUserID), Nickname: "Aegis", Username: "aegis", Status: userdomain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}
	return &userapp.ListUsersResult{Items: items, Page: 1, PageSize: 10, Total: 1}, nil
}

func assertAuthFailureEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantCode response.Code) {
	t.Helper()
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != wantCode {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func signRouteAuthToken(t *testing.T, secret, userID string) string {
	return signRouteAuthTokenWithVersion(t, secret, userID, 1)
}

func signRouteAuthTokenWithVersion(t *testing.T, secret, userID string, tokenVersion int64) string {
	t.Helper()
	if _, err := uuid.Parse(userID); err != nil {
		t.Fatalf("parse userID: %v", err)
	}
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, commonauth.Claims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        "s-123",
		RegisteredClaims: jwtv5.RegisteredClaims{Subject: commonauth.SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
