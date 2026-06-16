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

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
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
			Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, Exporter: "none"},
		},
	}
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)
	jwtService := commonauth.NewJWTService(cfg.Auth)
	tokenVersions := &routeTokenVersionValidator{version: 1}
	authorizer := &routeAuthorizer{allowed: true}
	traceProvider := newRouteTestTracingProvider(t, cfg)
	engine, err := NewGinEngine(GinParams{Config: cfg, Log: log, Trace: traceProvider})
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
		Authorizer:    authorizer,
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
		{method: http.MethodGet, path: "/livez"},
		{method: http.MethodGet, path: "/readyz"},
		{method: http.MethodGet, path: "/startupz"},
		{method: http.MethodGet, path: "/openapi/index.html"},
		{method: http.MethodGet, path: "/openapi.json"},
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
			if tt.path == "/api/v1/auth/change-password" {
				request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+"password-change-token")
			}
			engine.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusUnauthorized {
				t.Fatalf("%s status = %d, want not unauthorized", tt.path, recorder.Code)
			}
		})
	}
	if authorizer.calls != 0 {
		t.Fatalf("public route authorizer calls = %d, want 0", authorizer.calls)
	}

	t.Run("probes return configured service name", func(t *testing.T) {
		initialLogCount := logs.Len()
		for _, path := range []string{"/livez", "/readyz", "/startupz"} {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			engine.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusOK)
			}
			var health struct {
				Status  string `json:"status"`
				Service string `json:"service"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &health); err != nil {
				t.Fatalf("unmarshal health response: %v", err)
			}
			if health.Status != "ok" || health.Service != cfg.App.Name {
				t.Fatalf("%s health = %#v, want configured service name", path, health)
			}
		}
		if logs.Len() != initialLogCount {
			t.Fatalf("request log count changed from %d to %d, want successful probes skipped", initialLogCount, logs.Len())
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
	if authorizer.calls != 0 {
		t.Fatalf("auth route authorizer calls = %d, want 0", authorizer.calls)
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
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		assertSuccessEnvelope(t, recorder)
		if authorizer.calls != 1 || authorizer.userID != routeAuthUserID || authorizer.pathTemplate != "/api/v1/users/:user_id" || authorizer.method != http.MethodGet {
			t.Fatalf("authorizer call = %#v", authorizer)
		}

		entries := logs.FilterMessage("http request completed").All()
		if len(entries) == 0 {
			t.Fatal("request log count = 0, want at least one")
		}
		fields := entries[len(entries)-1].ContextMap()
		if !validTraceIDField(fields[logger.TraceIDField]) || fields["method"] != http.MethodGet || fields["path"] != "/api/v1/users/:user_id" || fields["status"] != int64(http.StatusOK) || fields[commonauth.UserIDKey] != routeAuthUserID {
			t.Fatalf("request log fields = %#v", fields)
		}
		if _, ok := fields["latency_ms"]; !ok {
			t.Fatalf("request log missing latency_ms: %#v", fields)
		}
		if _, ok := fields["client_ip"]; !ok {
			t.Fatalf("request log missing client_ip: %#v", fields)
		}
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
		if authorizer.calls != 1 {
			t.Fatalf("authorizer calls = %d, want 1", authorizer.calls)
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
		engine.GET("/panic-route-chain", func(_ *gin.Context) { panic("route-chain boom") })
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/panic-route-chain", nil)
		engine.ServeHTTP(recorder, request)

		assertFailureEnvelope(t, recorder, http.StatusInternalServerError, contracterrors.CodeInternalError)
		entries := logs.FilterMessage("panic recovered").All()
		if len(entries) != 1 {
			t.Fatalf("panic recovered logs = %d, want 1", len(entries))
		}
		fields := entries[0].ContextMap()
		if !validTraceIDField(fields[logger.TraceIDField]) || fields["panic"] != "route-chain boom" {
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

func newRouteTestTracingProvider(t *testing.T, cfg *config.Config) *commontracing.Provider {
	t.Helper()
	provider, err := commontracing.NewProvider(context.Background(), commontracing.Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown tracing provider: %v", err)
		}
	})
	return provider
}

func validTraceIDField(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	traceID, err := oteltrace.TraceIDFromHex(text)
	return err == nil && traceID.IsValid()
}

type routeAuthUserCommands struct{}

type routeAuthUserQueries struct{}

type routeAuthAuthUseCases struct{}

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
	items := []userdomain.User{{UserID: uuid.MustParse(routeAuthUserID), Nickname: "Aegis", Username: "aegis", Status: identity.UserStatusNormal, CreatedAt: now, UpdatedAt: now}}
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
	tokenID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("NewV7: %v", err)
	}
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, commonauth.Claims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        "s-123",
		RegisteredClaims: jwtv5.RegisteredClaims{ID: tokenID.String(), Subject: commonauth.SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
