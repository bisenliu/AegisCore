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
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const routeAuthUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

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

	handleHTTPServeError(zap.New(core), shutdowner, serveErr)

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

	handleHTTPServeError(zap.New(core), shutdowner, errors.New("serve failed"))

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

	handleHTTPServeError(zap.New(core), shutdowner, http.ErrServerClosed)
	handleHTTPServeError(zap.New(core), shutdowner, nil)

	if shutdowner.calls != 0 {
		t.Fatalf("shutdown calls = %d, want 0", shutdowner.calls)
	}
	if logs.Len() != 0 {
		t.Fatalf("error logs = %d, want 0", logs.Len())
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
		App: config.AppConfig{Environment: "local"},
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{Secret: "secret"},
		},
	}
	log := zap.NewNop()
	jwtService := auth.NewJWTService(cfg.Auth)
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
		AuthController: controller.NewAuthController(&routeAuthAuthService{}, validator),
		UserController: controller.NewUserController(&routeAuthUserService{}, validator),
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
				request.Header.Set(auth.AuthorizationHeader, auth.TokenPrefix+"password-change-token")
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
		request.Header.Set(auth.AuthorizationHeader, auth.TokenPrefix+"invalid")
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeTokenInvalid)
	})

	t.Run("query with valid token keeps controller behavior", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(auth.AuthorizationHeader, auth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		request.Header.Set("X-Trace-ID", "trace-auth-test")
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if recorder.Header().Get("X-Trace-ID") != "trace-auth-test" {
			t.Fatalf("X-Trace-ID = %q, want trace-auth-test", recorder.Header().Get("X-Trace-ID"))
		}
	})

	t.Run("query with mismatched token version returns token invalid", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(auth.AuthorizationHeader, auth.TokenPrefix+signRouteAuthTokenWithVersion(t, "secret", routeAuthUserID, 2))
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeTokenInvalid)
	})

	t.Run("query validation still runs after auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
		request.Header.Set(auth.AuthorizationHeader, auth.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	})
}

type routeAuthUserService struct{}

type routeAuthAuthService struct{}

type routeAuthSessionRepository struct {
	version int64
}

type routeTokenVersionValidator struct {
	version int64
}

func (s *routeTokenVersionValidator) ValidateTokenVersion(_ context.Context, _ string, tokenVersion int64) error {
	if tokenVersion != s.version {
		return errors.New("token version mismatch")
	}
	return nil
}

func (s *routeAuthSessionRepository) GetCachedTokenVersion(context.Context, string) (int64, error) {
	return s.version, nil
}

func (s *routeAuthSessionRepository) CacheTokenVersion(context.Context, string, int64) error {
	return nil
}

func (s *routeAuthSessionRepository) CreateSession(context.Context, repository.AuthSession, time.Duration) error {
	return nil
}

func (s *routeAuthSessionRepository) GetSession(context.Context, string) (repository.AuthSession, error) {
	return repository.AuthSession{}, nil
}

func (s *routeAuthSessionRepository) DeleteSession(context.Context, string, string) error {
	return nil
}

func (s *routeAuthSessionRepository) DeleteAllUserSessions(context.Context, string) error {
	return nil
}

func (s *routeAuthSessionRepository) InvalidateUserTokenVersion(context.Context, string) error {
	return nil
}

func (s *routeAuthAuthService) Login(context.Context, dto.LoginRequest) (*dto.TokenResponse, error) {
	return &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthService) Refresh(context.Context, dto.RefreshTokenRequest) (*dto.TokenResponse, error) {
	return &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthService) ChangePassword(context.Context, dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	return &dto.ChangePasswordResponse{Changed: true}, nil
}

func (s *routeAuthAuthService) Logout(context.Context) (*dto.LogoutResponse, error) {
	return &dto.LogoutResponse{LoggedOut: true}, nil
}

func (s *routeAuthAuthService) LogoutAll(context.Context) (*dto.LogoutResponse, error) {
	return &dto.LogoutResponse{LoggedOut: true}, nil
}

func (s *routeAuthUserService) CreateUser(context.Context, dto.CreateUserRequest) (*dto.UserResponse, error) {
	now := time.Now().UnixMilli()
	return &dto.UserResponse{UserID: routeAuthUserID, Nickname: "Alice", Username: "alice", Status: domain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *routeAuthUserService) GetUserByID(_ context.Context, userID uuid.UUID) (*dto.UserResponse, error) {
	userIDString := userID.String()
	if userIDString == "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3999" {
		return nil, response.NotFoundError("user not found")
	}
	if userIDString == "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3500" {
		return nil, errors.New("database down")
	}
	now := time.Now().UnixMilli()
	return &dto.UserResponse{UserID: userIDString, Nickname: "Aegis", Username: "aegis", Status: domain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *routeAuthUserService) ListUsers(context.Context, dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error) {
	now := time.Now().UnixMilli()
	items := []dto.UserResponse{{UserID: routeAuthUserID, Nickname: "Aegis", Username: "aegis", Status: domain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}
	return response.NewPaginatedData(items, response.NewPagination(1, 10, 1)), nil
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
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, auth.Claims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        "s-123",
		RegisteredClaims: jwtv5.RegisteredClaims{Subject: auth.SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
