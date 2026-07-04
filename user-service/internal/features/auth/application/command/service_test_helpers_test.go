package command

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
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

func newAuthCommandFixture(t testing.TB, authCfg config.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	t.Helper()
	return newAuthCommandFixtureWithController(gomock.NewController(t), authCfg, metrics)
}

func newAuthCommandFixtureWithController(ctrl *gomock.Controller, authCfg config.AuthConfig, metrics authapplication.Metrics) *authCommandFixture {
	credentials := NewMockVerifier(ctrl)
	tokens := NewMockIssuer(ctrl)
	sessions := NewMockLifecycle(ctrl)
	cfg := &config.Config{Auth: authCfg}
	return &authCommandFixture{
		testAuthUseCases: testAuthUseCases{
			LoginUseCase: NewLoginUseCase(LoginDeps{
				Credentials: credentials,
				Tokens:      tokens,
				Sessions:    sessions,
				Metrics:     metrics,
			}),
			RefreshTokenUseCase: NewRefreshTokenUseCase(RefreshTokenDeps{
				Tokens:   tokens,
				Sessions: sessions,
				Config:   cfg,
				Metrics:  metrics,
			}),
			ChangePasswordUseCase: NewChangePasswordUseCase(ChangePasswordDeps{
				Credentials: credentials,
				Tokens:      tokens,
				Sessions:    sessions,
			}),
			LogoutCurrentSessionUseCase: NewLogoutCurrentSessionUseCase(LogoutCurrentSessionDeps{
				Sessions: sessions,
				Metrics:  metrics,
			}),
			LogoutAllSessionsUseCase: NewLogoutAllSessionsUseCase(LogoutAllSessionsDeps{
				Sessions: sessions,
				Metrics:  metrics,
			}),
		},
		credentials: credentials,
		tokens:      tokens,
		sessions:    sessions,
	}
}

func defaultAuthConfig(rotation bool) config.AuthConfig {
	return config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: rotation, TokenVersionCacheTTL: time.Minute, MaxActiveSessionsPerUser: 5}
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

func refreshClaims(sessionID string, tokenVersion int64) *commonauth.Claims {
	return &commonauth.Claims{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}

func passwordChangeClaims(sessionID string, tokenVersion int64) *commonauth.Claims {
	return &commonauth.Claims{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}

func testPasswordService(t testing.TB) *password.Service {
	t.Helper()
	service, err := password.NewService(password.Options{Concurrency: 1, QueueSize: 1})
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
