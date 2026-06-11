package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const routeAuthUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
const routeAuthForbiddenUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3403"
const routeAuthNotFoundUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3999"
const routeAuthInternalErrorUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3500"

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
		Config:        cfg,
		Log:           log,
		Engine:        engine,
		JWT:           jwtService,
		TokenVersions: tokenVersions,
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

	t.Run("panic recovery returns envelope and logs trace id", func(t *testing.T) {
		engine.GET("/panic-route-chain", func(c *gin.Context) { panic("route-chain boom") })
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/panic-route-chain", nil)
		request.Header.Set("X-Trace-ID", "trace-panic-route-chain")
		engine.ServeHTTP(recorder, request)

		assertFailureEnvelope(t, recorder, http.StatusInternalServerError, contracterrors.CodeInternalError)
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

type routeAuthUserCommands struct{}

type routeAuthUserQueries struct{}

type routeAuthAuthUseCases struct{}

type routeTokenVersionValidator struct {
	version int64
}

func (s *routeTokenVersionValidator) ValidateTokenVersion(_ context.Context, _ string, tokenVersion int64) error {
	if tokenVersion != s.version {
		return &commonauth.TokenVersionMismatchError{Current: s.version, Token: tokenVersion}
	}
	return nil
}

func (s *routeAuthAuthUseCases) Login(context.Context, authcommand.LoginCommand) (*authcommand.TokenResult, error) {
	return &authcommand.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 3600}, nil
}

func (s *routeAuthAuthUseCases) Refresh(context.Context, authcommand.RefreshTokenCommand) (*authcommand.TokenResult, error) {
	return &authcommand.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 3600}, nil
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
	return &usercommand.CreateUserResult{User: userdomain.User{UserID: uuid.MustParse(routeAuthUserID), Nickname: "Alice", Username: "alice", Status: userdomain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}, nil
}

func (s *routeAuthUserQueries) GetUserByID(_ context.Context, req userquery.GetUserByIDQuery) (*userquery.GetUserResult, error) {
	userID := req.UserID
	userIDString := userID.String()
	if userIDString == routeAuthNotFoundUserID {
		return nil, userdomain.ErrUserNotFound
	}
	if userIDString == routeAuthForbiddenUserID {
		return nil, contracterrors.ForbiddenError("forbidden")
	}
	if userIDString == routeAuthInternalErrorUserID {
		return nil, errors.New("database down")
	}
	now := time.Now().UnixMilli()
	return &userquery.GetUserResult{User: userdomain.User{UserID: userID, Nickname: "Aegis", Username: "aegis", Status: userdomain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}, nil
}

func assertSuccessEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !envelope.Success || envelope.Code != contracterrors.CodeOK {
		t.Fatalf("envelope = %#v, want success OK", envelope)
	}
}

func assertFailureEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode contracterrors.Code) {
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

func (s *routeAuthUserQueries) ListUsers(context.Context, userquery.ListUsersQuery) (*userquery.ListUsersResult, error) {
	now := time.Now().UnixMilli()
	items := []userdomain.User{{UserID: uuid.MustParse(routeAuthUserID), Nickname: "Aegis", Username: "aegis", Status: userdomain.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}
	return &userquery.ListUsersResult{Items: items, PageSize: 10}, nil
}

func assertAuthFailureEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantCode contracterrors.Code) {
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
