package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/credentials"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/service"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const routeAuthUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

type lifecycleRecorder struct {
	hooks []fx.Hook
}

func (r *lifecycleRecorder) Append(hook fx.Hook) {
	r.hooks = append(r.hooks, hook)
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
	if got := strings.Join(cfg.Auth.Whitelist, ","); got != "/healthz,/swagger,/docs,/api-docs,/api/v1/auth/login,/api/v1/auth/refresh,/api/v1/auth/change-password" {
		t.Fatalf("Auth.Whitelist = %q, want default auth whitelist", got)
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

func TestGinEngineAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SWAGGER_ENABLED", "true")

	cfg := &config.Config{
		App: config.AppConfig{Environment: "local"},
		Auth: config.AuthConfig{
			JWT:       config.JWTConfig{Secret: "secret"},
			Whitelist: []string{"/healthz", "/swagger", "/docs", "/api-docs", "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/change-password"},
		},
	}
	engine, err := NewGinEngine(GinParams{Config: cfg, Log: zap.NewNop(), JWT: credentials.NewJWTService(cfg.Auth), SessionStore: &routeAuthSessionStore{version: 1}})
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	RegisterRoutes(RegisterRouteParams{
		Config:         cfg,
		Engine:         engine,
		AuthController: controller.NewAuthController(&routeAuthAuthService{}, validator),
		UserController: controller.NewUserController(&routeAuthUserService{}, validator),
	})

	publicPaths := []string{"/healthz", "/swagger/index.html", "/docs", "/api-docs", "/api/v1/auth/change-password"}
	for _, path := range publicPaths {
		t.Run("public "+path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			engine.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusUnauthorized {
				t.Fatalf("%s status = %d, want not unauthorized", path, recorder.Code)
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

	t.Run("query with invalid token returns token invalid", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(credentials.AuthorizationHeader, credentials.TokenPrefix+"invalid")
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeTokenInvalid)
	})

	t.Run("query with valid token keeps controller behavior", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+routeAuthUserID, nil)
		request.Header.Set(credentials.AuthorizationHeader, credentials.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
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
		request.Header.Set(credentials.AuthorizationHeader, credentials.TokenPrefix+signRouteAuthTokenWithVersion(t, "secret", routeAuthUserID, 2))
		engine.ServeHTTP(recorder, request)
		assertAuthFailureEnvelope(t, recorder, response.CodeTokenInvalid)
	})

	t.Run("query validation still runs after auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
		request.Header.Set(credentials.AuthorizationHeader, credentials.TokenPrefix+signRouteAuthToken(t, "secret", routeAuthUserID))
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	})
}

type routeAuthUserService struct{}

type routeAuthAuthService struct{}

type routeAuthSessionStore struct {
	version int64
}

func (s *routeAuthSessionStore) GetCurrentTokenVersion(context.Context, string) (int64, error) {
	return s.version, nil
}

func (s *routeAuthSessionStore) ValidateTokenVersion(_ context.Context, _ string, tokenVersion int64) error {
	if tokenVersion != s.version {
		return errors.New("token version mismatch")
	}
	return nil
}

func (s *routeAuthSessionStore) CreateSession(context.Context, service.Session, time.Duration) error {
	return nil
}

func (s *routeAuthSessionStore) GetSession(context.Context, string) (service.Session, error) {
	return service.Session{}, nil
}

func (s *routeAuthSessionStore) DeleteSession(context.Context, string, string) error {
	return nil
}

func (s *routeAuthSessionStore) DeleteAllUserSessions(context.Context, string) error {
	return nil
}

func (s *routeAuthSessionStore) InvalidateUserTokenVersion(context.Context, string) error {
	return nil
}

func (s *routeAuthAuthService) Login(context.Context, dto.LoginRequest) (*dto.TokenResponse, error) {
	return &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: credentials.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthService) Refresh(context.Context, dto.RefreshTokenRequest) (*dto.TokenResponse, error) {
	return &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: credentials.TokenTypeBearer, ExpiresIn: 3600}, nil
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

func (s *routeAuthUserService) GetUserByID(_ context.Context, userID string) (*dto.UserResponse, error) {
	if userID == "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3999" {
		return nil, response.NotFoundError("user not found")
	}
	if userID == "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3500" {
		return nil, errors.New("database down")
	}
	now := time.Now().UnixMilli()
	return &dto.UserResponse{UserID: userID, Nickname: "Aegis", Username: "aegis", Status: domain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}, nil
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
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, credentials.Claims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        "s-123",
		RegisteredClaims: jwtv5.RegisteredClaims{Subject: credentials.SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
