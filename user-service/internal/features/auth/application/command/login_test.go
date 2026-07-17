package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var authTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestAuthUseCaseLogin(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	credential := normalCredential()
	issued := issuedTokenPair("access", "refresh", 900, time.Hour)

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(credential, nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ int64, sessionID string) (*authtokens.IssuedTokenPair, error) {
			require.NotEqual(t, "", sessionID,
				"sessionID is empty")

			return issued, nil
		})
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), time.Hour).
		DoAndReturn(func(_ context.Context, _ string, sessionID string, _ int64, _ time.Duration) error {
			require.NotEqual(t, "", sessionID,
				"sessionID is empty")

			return nil
		})

	result, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	require.NoError(t, err,
		"Login: %v", err)
	require.False(t, result.PasswordChangeRequired)
	require.False(t, result.Tokens.AccessToken != "access" || result.Tokens.RefreshToken != "refresh" || result.Tokens.TokenType != commonauth.TokenTypeBearer || result.Tokens.ExpiresIn != 900,
		"tokens = %#v", result.Tokens)

}

func TestAuthUseCaseLoginRecordsMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", 900, time.Hour), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), time.Hour).Return(nil)
	metrics.EXPECT().LoginSucceeded(gomock.Any())

	_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	require.NoError(t, err,
		"Login: %v", err)

}

func TestAuthUseCaseLoginRecordsFailureReasons(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		metrics := NewMockMetrics(ctrl)
		fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
		metrics.EXPECT().LoginFailed(gomock.Any(), authapplication.MetricsReasonValidationFailed)

		_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: " "})
		require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
			"err = %v, want invalid credentials", err)

	})

	t.Run("status rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		metrics := NewMockMetrics(ctrl)
		fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
		fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(nil, errors.Join(authdomain.ErrInvalidCredentials, authdomain.ErrUserStatusRejected))
		metrics.EXPECT().LoginFailed(gomock.Any(), authapplication.MetricsReasonUserStatusRejected)

		_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
		require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
			"err = %v, want invalid credentials", err)

	})

	t.Run("kdf busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		metrics := NewMockMetrics(ctrl)
		fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
		fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(nil, password.ErrPasswordKDFBusy)
		metrics.EXPECT().LoginFailed(gomock.Any(), authapplication.MetricsReasonPasswordKDFBusy)

		result, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
		require.ErrorIs(t, err, password.ErrPasswordKDFBusy,
			"err = %v, want ErrPasswordKDFBusy", err)
		require.Nil(t, result,
			"result = %#v, want nil", result)

	})
}

func TestAuthUseCaseLoginRejectsBlankTrimmedCredentials(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)

	_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: " "})
	require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
		"err = %v, want authdomain.ErrInvalidCredentials", err)

}

func TestAuthUseCaseLoginUsesDefaultTTLs(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	defaultAccessTokenTTL := 15 * time.Minute
	defaultRefreshTokenTTL := 7 * 24 * time.Hour

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", int64(defaultAccessTokenTTL.Seconds()), defaultRefreshTokenTTL), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), defaultRefreshTokenTTL).Return(nil)

	result, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	require.NoError(t, err,
		"Login: %v", err)
	require.False(t, result.PasswordChangeRequired)
	require.Equal(t, int64(defaultAccessTokenTTL.Seconds()), result.Tokens.ExpiresIn,
		"ExpiresIn = %d, want %d", result.Tokens.ExpiresIn, int64(defaultAccessTokenTTL.Seconds()))

}

func TestAuthUseCaseLoginUsesExplicitTTLs(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	accessTTL := time.Minute
	refreshTTL := 2 * time.Hour

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", int64(accessTTL.Seconds()), refreshTTL), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), refreshTTL).Return(nil)

	result, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	require.NoError(t, err,
		"Login: %v", err)
	require.False(t, result.PasswordChangeRequired)
	require.Equal(t, int64(accessTTL.Seconds()), result.Tokens.ExpiresIn,
		"ExpiresIn = %d, want %d", result.Tokens.ExpiresIn, int64(accessTTL.Seconds()))

}

func TestAuthUseCaseLoginPassesMaxActiveSessionsPerUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	credentials := NewMockVerifier(ctrl)
	tokens := NewMockIssuer(ctrl)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenVersions := NewMockTokenVersionCache(ctrl)
	sessions := NewMockRefreshSessionStore(ctrl)
	passwordChanges := NewMockPasswordChangeSessionStore(ctrl)
	cfg := serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: true, TokenVersionCacheTTL: time.Minute, MaxActiveSessionsPerUser: 3}
	svc := NewLoginUseCase(credentials, tokens, authsessions.NewLifecycle(users, tokenVersions, sessions, passwordChanges, cfg.MaxActiveSessionsPerUser, noopTokenVersionInvalidator{}), nil)

	credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", 900, time.Hour), nil)
	sessions.EXPECT().CreateSession(gomock.Any(), gomock.Any(), time.Hour, 3).DoAndReturn(func(_ context.Context, session authdomain.AuthSession, _ time.Duration, _ int) error {
		require.False(t, session.UserID != authTestUserID.String() || session.SessionID == "" || session.TokenVersion != 2,
			"session = %#v", session)

		return nil
	})

	_, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	require.NoError(t, err,
		"Login: %v", err)

}

func TestAuthUseCaseLoginDoesNotReturnTokenWhenSessionCreateFails(t *testing.T) {
	createErr := errors.New("create session failed")
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(normalCredential(), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh", 900, time.Hour), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), gomock.Any(), int64(2), time.Hour).Return(createErr)

	result, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	require.ErrorIs(t, err, createErr,
		"Login err = %v, want create session error", err)
	require.Nil(t, result,
		"result = %#v, want nil", result)

}

func TestAuthUseCaseLoginRejectsInvalidCredentials(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "wrong").Return(nil, authdomain.ErrInvalidCredentials)

	_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "wrong"})
	require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
		"err = %v, want authdomain.ErrInvalidCredentials", err)

}

func TestAuthUseCaseLoginRejectsInactiveStatuses(t *testing.T) {
	for _, status := range []identity.UserStatus{identity.UserStatusDisabled} {
		fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
		fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(nil, errors.Join(authdomain.ErrInvalidCredentials, authdomain.ErrUserStatusRejected))

		_, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
		require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
			"status %d err = %v, want authdomain.ErrInvalidCredentials", status, err)

	}
}

func TestAuthUseCaseLoginIssuesPasswordChangeToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	credential := &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", Status: identity.UserStatusMustChangePassword, TokenVersion: 2}
	passwordChange := &authtokens.TokenResult{AccessToken: "password-change", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}
	claims := passwordChangeClaims("pc-123", 2)

	fixture.credentials.EXPECT().VerifyPassword(gomock.Any(), "alice", "secret").Return(credential, nil)
	fixture.tokens.EXPECT().IssuePasswordChangeToken(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(passwordChange, nil)
	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().CreatePasswordChangeSession(gomock.Any(), authTestUserID.String(), gomock.Any(), "jti-123", int64(2), 15*time.Minute).Return(nil)

	result, err := fixture.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})
	require.NoError(t, err,
		"Login: %v", err)
	require.True(t, result.PasswordChangeRequired)
	require.False(t, result.Tokens.AccessToken != "password-change" || result.Tokens.RefreshToken != "",
		"tokens = %#v", result.Tokens)

}
