package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestAuthUseCaseRefreshRotatesSession(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	gomock.InOrder(
		fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "Bearer refresh").Return(claims, nil),
		fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil),
		fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil),
		fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).DoAndReturn(func(_ context.Context, _ authdomain.AuthSession, newSession authdomain.AuthSession, _ time.Duration) error {
			require.False(t, newSession.UserID != authTestUserID.String() || newSession.SessionID == "" || newSession.SessionID == "s-old" || newSession.TokenVersion != 2,
				"new session = %#v", newSession)

			return nil
		}),
	)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: commonauth.TokenPrefix + "refresh"})
	require.NoError(t, err,
		"Refresh: %v", err)
	require.False(t, tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new",
		"tokens = %#v", tokens)

}

func TestAuthUseCaseRefreshRotationPassesMaxActiveSessionsPerUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokens := NewMockIssuer(ctrl)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenVersions := NewMockTokenVersionCache(ctrl)
	sessions := NewMockRefreshSessionStore(ctrl)
	cfg := config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, TokenVersionCacheTTL: time.Minute, RefreshTokenRotation: true, MaxActiveSessionsPerUser: 4}
	svc := NewRefreshTokenUseCase(RefreshTokenDeps{
		Tokens:   tokens,
		Sessions: authsessions.NewLifecycle(users, tokenVersions, sessions, cfg.MaxActiveSessionsPerUser),
		Config:   &config.Config{Auth: cfg},
	})
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	sessions.EXPECT().GetSession(gomock.Any(), authTestUserID.String(), "s-old").Return(oldSession, nil)
	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(2), nil)
	tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	sessions.EXPECT().RotateSession(gomock.Any(), oldSession, gomock.Any(), time.Hour, 4).Return(nil)

	_, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.NoError(t, err,
		"Refresh: %v", err)

}

func TestAuthUseCaseRefreshRejectsInvalidNormalizedToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(false), nil)

	for _, token := range []string{"", " ", commonauth.TokenTypeBearer, commonauth.TokenPrefix} {
		_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: token})
		require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
			"token %q err = %v, want authdomain.ErrTokenInvalid", token, err)

	}
}

func TestAuthUseCaseRefreshRotationKeepsOldSessionWhenTokenSigningFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	signErr := errors.New("sign failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(nil, signErr)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, signErr,
		"Refresh err = %v, want sign error", err)

}

func TestAuthUseCaseRefreshRotationKeepsOldSessionWhenNewSessionCreateFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	createErr := errors.New("create failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(createErr)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, createErr,
		"Refresh err = %v, want create error", err)
	require.Nil(t, tokens,
		"tokens = %#v, want nil", tokens)

}

func TestAuthUseCaseRefreshRotationFailureDoesNotReturnToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	rotateErr := errors.New("rotate failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(rotateErr)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, rotateErr,
		"Refresh err = %v, want rotate error", err)
	require.Nil(t, tokens,
		"tokens = %#v, want nil", tokens)

}

func TestAuthUseCaseRefreshRotationReturnsTokenAfterNewSessionAndOldRevocation(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(nil)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.NoError(t, err,
		"Refresh: %v", err)
	require.False(t, tokens == nil || tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new",
		"tokens = %#v", tokens)

}

func TestAuthUseCaseRefreshUsesNormalizedToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(false), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID.String(), int64(2), "s-old").Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID.String(), "s-old", int64(2), time.Hour).Return(nil)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.NoError(t, err,
		"Refresh: %v", err)
	require.False(t, tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new",
		"tokens = %#v", tokens)

}

func TestAuthUseCaseRefreshRejectsAccessTokenSubject(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(false), nil)
	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "access").Return(nil, authdomain.ErrTokenInvalid)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "access"})
	require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
		"err = %v, want authdomain.ErrTokenInvalid", err)

}

func TestAuthUseCaseRefreshRejectsVersionChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	claims := refreshClaims("s-old", 2)

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(authdomain.AuthSession{}, int64(0), errors.Join(authdomain.ErrTokenInvalid, commonauth.ErrTokenVersionMismatch))
	metrics.EXPECT().RefreshFailed(gomock.Any(), authapplication.MetricsReasonTokenVersionMismatch)
	metrics.EXPECT().TokenVersionMismatch(gomock.Any(), authapplication.MetricsSourceRefreshToken)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
		"err = %v, want authdomain.ErrTokenInvalid", err)

}

func TestAuthUseCaseRefreshMapsTokenVersionUserNotFound(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(authdomain.AuthSession{}, int64(0), identity.ErrUserNotFound)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}
