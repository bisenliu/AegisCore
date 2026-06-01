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
	commonjwt "github.com/aegiscore/common/jwt"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

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
	if got := strings.Join(cfg.Auth.Whitelist, ","); got != "/healthz,/swagger,/docs,/api-docs" {
		t.Fatalf("Auth.Whitelist = %q, want /healthz,/swagger,/docs,/api-docs", got)
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
			Whitelist: []string{"/healthz", "/swagger", "/docs", "/api-docs"},
		},
	}
	engine, err := NewGinEngine(GinParams{Config: cfg, Log: zap.NewNop(), JWT: commonjwt.NewService(cfg.Auth)})
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
		UserController: controller.NewUserController(&routeAuthUserService{}, validator),
	})

	publicPaths := []string{"/healthz", "/swagger/index.html", "/docs", "/api-docs"}
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
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
		engine.ServeHTTP(recorder, request)
		assertUnauthenticatedEnvelope(t, recorder)
	})

	t.Run("create requires auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		assertUnauthenticatedEnvelope(t, recorder)
	})

	t.Run("query with valid token keeps controller behavior", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
		request.Header.Set("Authorization", "Bearer "+signRouteAuthToken(t, "secret", "u-123"))
		request.Header.Set("X-Trace-ID", "trace-auth-test")
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if recorder.Header().Get("X-Trace-ID") != "trace-auth-test" {
			t.Fatalf("X-Trace-ID = %q, want trace-auth-test", recorder.Header().Get("X-Trace-ID"))
		}
	})

	t.Run("query validation still runs after auth", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
		request.Header.Set("Authorization", "Bearer "+signRouteAuthToken(t, "secret", "u-123"))
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	})
}

type routeAuthUserService struct{}

func (s *routeAuthUserService) CreateUser(context.Context, dto.CreateUserRequest) (*dto.UserResponse, error) {
	return &dto.UserResponse{ID: 124, Name: "Alice", Email: "alice@example.com", Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (s *routeAuthUserService) GetUserByID(_ context.Context, id int64) (*dto.UserResponse, error) {
	if id == 999 {
		return nil, response.NotFoundError("user not found")
	}
	if id == 500 {
		return nil, errors.New("database down")
	}
	return &dto.UserResponse{ID: id, Name: "Aegis", Email: "aegis@example.com", Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func assertUnauthenticatedEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != response.CodeUnauthenticated {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func signRouteAuthToken(t *testing.T, secret, userID string) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, commonjwt.Claims{
		UserID:           userID,
		RegisteredClaims: jwtv5.RegisteredClaims{ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
