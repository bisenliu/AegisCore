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
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestAuthUseCaseRefreshRotatesSession(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}

	gomock.InOrder(
		fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "Bearer refresh").Return(claims, authTestUserID, nil),
		fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil),
		fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil),
		fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).DoAndReturn(func(_ context.Context, _ authdomain.AuthSession, newSession authdomain.AuthSession, _ time.Duration) error {
			require.False(t, newSession.UserID != authTestUserID || newSession.SessionID == "" || newSession.SessionID == "s-old" || newSession.TokenVersion != 2,
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
	passwordChanges := NewMockPasswordChangeSessionStore(ctrl)
	cfg := serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, TokenVersionCacheTTL: time.Minute, RefreshTokenRotation: true, MaxActiveSessionsPerUser: 4}
	lifecycle, err := authsessions.NewLifecycle(users, tokenVersions, sessions, passwordChanges, cfg.MaxActiveSessionsPerUser, noopTokenVersionInvalidator{})
	require.NoError(t, err)
	svc := NewRefreshTokenUseCase(tokens, lifecycle, nil, RefreshTokenSettings{RefreshTokenRotation: cfg.RefreshTokenRotation})
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}

	tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
	sessions.EXPECT().GetSession(gomock.Any(), authTestUserID, "s-old").Return(oldSession, nil)
	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID).Return(int64(2), nil)
	tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	sessions.EXPECT().RotateSession(gomock.Any(), oldSession, gomock.Any(), time.Hour, 4).Return(nil)

	_, err = svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
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
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}
	signErr := errors.New("sign failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(nil, signErr)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, signErr,
		"Refresh err = %v, want sign error", err)

}

func TestAuthUseCaseRefreshRotationKeepsOldSessionWhenNewSessionCreateFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}
	createErr := errors.New("create failed")

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
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
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}
	rotateErr := errors.New("rotate failed")
	issued := issuedTokenPair("access-visible-if-bug", "refresh-visible-if-bug", 900, time.Hour)

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issued, nil)
	fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(rotateErr)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, rotateErr,
		"Refresh err = %v, want rotate error", err)
	require.Nil(t, tokens,
		"IssueTokenPair returned %#v before rotation failed; Refresh must not expose it", issued.Result)

}

func TestAuthUseCaseRefreshRotationFailureKeepsReplayWindowOnOldSession(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}
	rotateErr := errors.New("rotate failed")

	gomock.InOrder(
		fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil),
		fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil),
		fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issuedTokenPair("access-lost", "refresh-lost", 900, time.Hour), nil),
		fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(rotateErr),
		fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil),
		fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil),
		fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issuedTokenPair("access-retry", "refresh-retry", 900, time.Hour), nil),
		fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(nil),
	)

	firstTokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, rotateErr,
		"first Refresh err = %v, want rotate error", err)
	require.Nil(t, firstTokens,
		"first tokens = %#v, want nil", firstTokens)

	retryTokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.NoError(t, err,
		"retry Refresh: %v", err)
	require.False(t, retryTokens == nil || retryTokens.AccessToken != "access-retry" || retryTokens.RefreshToken != "refresh-retry",
		"retry tokens = %#v", retryTokens)

}

func TestAuthUseCaseRefreshRotationRejectsReplayedOldRefreshAfterSuccess(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}

	gomock.InOrder(
		fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil),
		fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil),
		fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issuedTokenPair("access-new", "refresh-new", 900, time.Hour), nil),
		fixture.sessions.EXPECT().RotateTokenSession(gomock.Any(), oldSession, gomock.Any(), time.Hour).Return(nil),
		fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil),
		fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(authdomain.AuthSession{}, int64(2), authdomain.ErrAuthSessionNotFound),
	)

	freshTokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.NoError(t, err,
		"first Refresh: %v", err)
	require.False(t, freshTokens == nil || freshTokens.RefreshToken != "refresh-new",
		"first tokens = %#v", freshTokens)

	replayTokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, authdomain.ErrAuthSessionNotFound,
		"replay err = %v, want ErrAuthSessionNotFound", err)
	require.Nil(t, replayTokens,
		"replay tokens = %#v, want nil", replayTokens)

}

func TestAuthUseCaseRefreshRotationReturnsTokenAfterNewSessionAndOldRevocation(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := refreshClaims("s-old", 2)
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), gomock.Any()).Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
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
	oldSession := authdomain.AuthSession{UserID: authTestUserID, SessionID: "s-old", TokenVersion: 2}

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(oldSession, int64(2), nil)
	fixture.tokens.EXPECT().IssueTokenPair(gomock.Any(), authTestUserID, int64(2), "s-old").Return(issuedTokenPair("access", "refresh-new", 900, time.Hour), nil)
	fixture.sessions.EXPECT().CreateTokenSession(gomock.Any(), authTestUserID, "s-old", int64(2), time.Hour).Return(nil)

	tokens, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.NoError(t, err,
		"Refresh: %v", err)
	require.False(t, tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new",
		"tokens = %#v", tokens)

}

func TestAuthUseCaseRefreshRejectsAccessTokenSubject(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(false), nil)
	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "access").Return(nil, uuid.Nil, authdomain.ErrTokenInvalid)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "access"})
	require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
		"err = %v, want authdomain.ErrTokenInvalid", err)

}

func TestAuthUseCaseRefreshRejectsVersionChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	claims := refreshClaims("s-old", 2)

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
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

	fixture.tokens.EXPECT().ParseRefreshToken(gomock.Any(), "refresh").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidateRefreshSession(gomock.Any(), claims).Return(authdomain.AuthSession{}, int64(0), identity.ErrUserNotFound)

	_, err := fixture.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}
