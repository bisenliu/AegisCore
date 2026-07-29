package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	oteltrace "go.opentelemetry.io/otel/trace"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	commonresources "github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/common/runtime/workerpool"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	providerobservability "github.com/aegiscore/user-service/internal/providers/observability"
	"github.com/aegiscore/user-service/internal/resources"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func newRouteTestTracingProvider(t *testing.T, cfg *config.Config) *commontracing.Provider {
	t.Helper()
	return newGinTestTracingProvider(t, cfg)
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

func registerRouteTestRuntimeMetrics(t *testing.T, provider *commonmetrics.Provider) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:route_runtime_metrics?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	resourceSettings := serviceconfig.ResourceSettings{
		Redis: commonresources.RedisConfigs{
			resources.NameCacheRedis: {Timeout: time.Second},
		},
	}

	err = providerobservability.RegisterRuntimeDependencyMetrics(providerobservability.RuntimeDependencyMetricsParams{
		Resources:        resourceSettings,
		Metrics:          provider,
		PrimaryDB:        db,
		CacheRedis:       client,
		SessionPurgePool: routePurgeTaskPool{stats: workerpool.Stats{Name: "auth.redis.session_purge", Workers: 4, Submitted: 1}},
		PolicyWatcher:    routeWatcherStatus{running: true},
		AuthTokenCache:   routeLocalcacheStatsSource{name: "auth_token_version", stats: localcache.Stats{Capacity: 1000}},
		RBACRolesCache:   routeLocalcacheStatsSource{name: "rbac_user_roles", stats: localcache.Stats{Capacity: 2000}},
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

type routeWatcherStatus struct {
	running bool
	err     error
}

type routeLocalcacheStatsSource struct {
	name  string
	stats localcache.Stats
}

func (p routePurgeTaskPool) Submit(context.Context, workerpool.Task) error {
	return nil
}

func (p routePurgeTaskPool) Stats() workerpool.Stats {
	return p.stats
}

func (s routeWatcherStatus) Running() bool {
	return s.running
}

func (s routeWatcherStatus) LastError() error {
	return s.err
}

func (s routeLocalcacheStatsSource) Name() string {
	return s.name
}

func (s routeLocalcacheStatsSource) Stats() localcache.Stats {
	return s.stats
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

func (s *routeAuthAuthUseCases) Login(context.Context, authcommand.LoginCommand) (*authcommand.LoginResult, error) {
	return &authcommand.LoginResult{Tokens: &authtokens.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 3600}}, nil
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
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, routeAuthClaims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        "s-123",
		RegisteredClaims: jwtv5.RegisteredClaims{ID: tokenID.String(), Subject: "access", ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}

type routeAuthClaims struct {
	UserID       string `json:"user_id"`
	TokenVersion int64  `json:"token_version"`
	SessionID    string `json:"session_id"`
	jwtv5.RegisteredClaims
}
