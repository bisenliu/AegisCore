package command

import (
	"context"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	platformauth "github.com/aegiscore/platform/security/auth"
	"github.com/aegiscore/platform/security/password"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

type testAuthUseCases struct {
	LoginUseCase
	RefreshTokenUseCase
	ForceChangePasswordUseCase
	LogoutCurrentSessionUseCase
	LogoutAllSessionsUseCase
}

type authCommandFixture struct {
	testAuthUseCases
	credentials *MockVerifier
	tokens      *MockIssuer
	sessions    *MockLifecycle
	lockout     *stubLoginLockout
	denylist    *stubAccessDenylist
}

type noopTokenVersionInvalidator struct{}

func (noopTokenVersionInvalidator) InvalidateTokenVersion(string) {}

func newAuthCommandFixture(t testing.TB, authCfg serviceconfig.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	t.Helper()
	return newAuthCommandFixtureWithController(gomock.NewController(t), authCfg, metrics)
}

func newAuthCommandFixtureWithController(ctrl *gomock.Controller, authCfg serviceconfig.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	credentials := NewMockVerifier(ctrl)
	tokens := NewMockIssuer(ctrl)
	sessions := NewMockLifecycle(ctrl)
	lockout := &stubLoginLockout{}
	denylist := &stubAccessDenylist{}
	login, err := NewLoginUseCase(credentials, tokens, sessions, lockout, authCfg.LoginLockout, metrics)
	if err != nil {
		panic(err)
	}
	return &authCommandFixture{
		testAuthUseCases: testAuthUseCases{
			LoginUseCase:                login,
			RefreshTokenUseCase:         NewRefreshTokenUseCase(tokens, sessions, metrics, RefreshTokenSettings{RefreshTokenRotation: authCfg.RefreshTokenRotation}),
			ForceChangePasswordUseCase:  NewForceChangePasswordUseCase(credentials, tokens, sessions, metrics),
			LogoutCurrentSessionUseCase: NewLogoutCurrentSessionUseCase(sessions, denylist, metrics),
			LogoutAllSessionsUseCase:    NewLogoutAllSessionsUseCase(sessions, metrics),
		},
		credentials: credentials,
		tokens:      tokens,
		sessions:    sessions,
		lockout:     lockout,
		denylist:    denylist,
	}
}

func defaultAuthConfig(rotation bool) serviceconfig.AuthConfig {
	return serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: rotation, TokenVersionCacheTTL: time.Minute, MaxActiveSessionsPerUser: 5, LoginLockout: serviceconfig.LoginLockoutConfig{Enabled: true, MaxFailures: 5, FailureWindow: 15 * time.Minute, LockoutTTL: 15 * time.Minute}}
}

// stubLoginLockout 用内存计数模拟登录锁定，测试不依赖 Redis。
type stubLoginLockout struct {
	failures      int
	locked        bool
	statusErr     error
	registerErr   error
	clearErr      error
	cleared       bool
	registered    int
	statusLookups int
}

func (s *stubLoginLockout) LoginLockoutStatus(context.Context, string) (authapplication.LoginLockoutStatus, error) {
	s.statusLookups++
	if s.statusErr != nil {
		return authapplication.LoginLockoutStatus{}, s.statusErr
	}
	return authapplication.LoginLockoutStatus{Locked: s.locked}, nil
}

func (s *stubLoginLockout) RegisterLoginFailure(context.Context, string, int, time.Duration, time.Duration) (authapplication.LoginLockoutStatus, error) {
	s.registered++
	if s.registerErr != nil {
		return authapplication.LoginLockoutStatus{}, s.registerErr
	}
	s.failures++
	if s.failures >= 5 {
		s.locked = true
	}
	return authapplication.LoginLockoutStatus{Locked: s.locked}, nil
}

// stubAccessDenylist 记录单会话登出是否写入当前 access token。
type stubAccessDenylist struct {
	denied    map[string]time.Duration
	denyErr   error
	deniedHit bool
	checkErr  error
}

func (s *stubAccessDenylist) DenyAccessToken(_ context.Context, tokenID string, ttl time.Duration) error {
	if s.denyErr != nil {
		return s.denyErr
	}
	if s.denied == nil {
		s.denied = map[string]time.Duration{}
	}
	s.denied[tokenID] = ttl
	return nil
}

func (s *stubAccessDenylist) AccessTokenDenied(_ context.Context, tokenID string) (bool, error) {
	if s.checkErr != nil {
		return false, s.checkErr
	}
	_, ok := s.denied[tokenID]
	return ok || s.deniedHit, nil
}

func (s *stubLoginLockout) ClearLoginFailures(context.Context, string) error {
	if s.clearErr != nil {
		return s.clearErr
	}
	s.failures = 0
	s.locked = false
	s.cleared = true
	return nil
}

func normalCredential() *authdomain.UserCredential {
	return &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", Status: identity.UserStatusNormal, TokenVersion: 2}
}

func issuedTokenPair(accessToken string, refreshToken string, expiresIn int64, refreshTTL time.Duration) *authtokens.IssuedTokenPair {
	return &authtokens.IssuedTokenPair{
		Result:     &authtokens.TokenResult{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: platformauth.TokenTypeBearer, ExpiresIn: expiresIn},
		RefreshTTL: refreshTTL,
	}
}

func refreshClaims(sessionID string, tokenVersion int64) *authtokens.Claims {
	return &authtokens.Claims{UserID: authTestUserID, SessionID: sessionID, TokenVersion: tokenVersion}
}

func passwordChangeClaims(sessionID string, tokenVersion int64) *authtokens.Claims {
	return &authtokens.Claims{UserID: authTestUserID, SessionID: sessionID, TokenVersion: tokenVersion, RegisteredClaims: jwtv5.RegisteredClaims{ID: "jti-123"}}
}

func testPasswordService(t testing.TB) *password.Service {
	t.Helper()
	service, err := password.NewService()
	require.NoError(t, err,
		"NewService: %v", err)

	return service
}

func hashTestPassword(t testing.TB, plain string) (string, error) {
	t.Helper()
	return testPasswordService(t).HashContext(context.Background(), plain)
}

func verifyTestPassword(t testing.TB, plain, encodedHash string) (bool, error) {
	t.Helper()
	return testPasswordService(t).VerifyContext(context.Background(), plain, encodedHash)
}
