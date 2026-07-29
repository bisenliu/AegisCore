package command

import (
	"context"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

type testAuthUseCases struct {
	LoginUseCase
	RefreshTokenUseCase
	ChangePasswordUseCase
	LogoutCurrentSessionUseCase
	LogoutAllSessionsUseCase
}

type authCommandFixture struct {
	testAuthUseCases
	credentials *MockVerifier
	tokens      *MockIssuer
	sessions    *MockLifecycle
}

type noopTokenVersionInvalidator struct{}

func (noopTokenVersionInvalidator) InvalidateTokenVersion(string) error { return nil }

func newAuthCommandFixture(t testing.TB, authCfg serviceconfig.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	t.Helper()
	return newAuthCommandFixtureWithController(gomock.NewController(t), authCfg, metrics)
}

func newAuthCommandFixtureWithController(ctrl *gomock.Controller, authCfg serviceconfig.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	credentials := NewMockVerifier(ctrl)
	tokens := NewMockIssuer(ctrl)
	sessions := NewMockLifecycle(ctrl)
	return &authCommandFixture{
		testAuthUseCases: testAuthUseCases{
			LoginUseCase:                NewLoginUseCase(credentials, tokens, sessions, metrics),
			RefreshTokenUseCase:         NewRefreshTokenUseCase(tokens, sessions, metrics, RefreshTokenSettings{RefreshTokenRotation: authCfg.RefreshTokenRotation}),
			ChangePasswordUseCase:       NewChangePasswordUseCase(credentials, tokens, sessions, metrics),
			LogoutCurrentSessionUseCase: NewLogoutCurrentSessionUseCase(sessions, metrics),
			LogoutAllSessionsUseCase:    NewLogoutAllSessionsUseCase(sessions, metrics),
		},
		credentials: credentials,
		tokens:      tokens,
		sessions:    sessions,
	}
}

func defaultAuthConfig(rotation bool) serviceconfig.AuthConfig {
	return serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: rotation, TokenVersionCacheTTL: time.Minute, MaxActiveSessionsPerUser: 5}
}

func normalCredential() *authdomain.UserCredential {
	return &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", Status: identity.UserStatusNormal, TokenVersion: 2}
}

func issuedTokenPair(accessToken string, refreshToken string, expiresIn int64, refreshTTL time.Duration) *authtokens.IssuedTokenPair {
	return &authtokens.IssuedTokenPair{
		Response:   &authtokens.TokenResult{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: commonauth.TokenTypeBearer, ExpiresIn: expiresIn},
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
