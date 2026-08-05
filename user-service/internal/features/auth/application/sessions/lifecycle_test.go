package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestLifecycleRotateTokenSessionMapsRejectedSession(t *testing.T) {
	for _, rejectedErr := range []error{authdomain.ErrAuthSessionNotFound, authdomain.ErrAuthSessionMismatch} {
		t.Run(rejectedErr.Error(), func(t *testing.T) {
			fixture := newLifecycleTestFixture(t)
			oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-old", TokenVersion: 2}
			newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 2}
			fixture.sessions.EXPECT().
				RotateSession(gomock.Any(), oldSession, newSession, time.Hour, 5).
				Return(rejectedErr)

			err := fixture.lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)
			require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
				"err = %v, want ErrTokenInvalid", err)
			require.ErrorIs(t, err, rejectedErr,
				"err = %v, want rejected session error", err)

		})
	}
}

func TestLifecycleConsumePasswordChangeClaimsMapsRejectedSession(t *testing.T) {
	for _, rejectedErr := range []error{authdomain.ErrPasswordChangeSessionNotFound, authdomain.ErrPasswordChangeSessionMismatch} {
		t.Run(rejectedErr.Error(), func(t *testing.T) {
			fixture := newLifecycleTestFixture(t)
			claims := &authtokens.Claims{
				UserID:       sessionTestUserID,
				SessionID:    "password-session",
				TokenVersion: 2,
				RegisteredClaims: jwtv5.RegisteredClaims{
					ID: "password-token",
				},
			}
			expected := authdomain.PasswordChangeSession{
				UserID:       claims.UserID,
				SessionID:    claims.SessionID,
				TokenID:      claims.ID,
				TokenVersion: claims.TokenVersion,
			}
			fixture.passwordChanges.EXPECT().
				ConsumePasswordChangeSession(gomock.Any(), expected).
				Return(rejectedErr)

			err := fixture.lifecycle.ConsumePasswordChangeClaims(context.Background(), claims)
			require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
				"err = %v, want ErrTokenInvalid", err)
			require.ErrorIs(t, err, rejectedErr,
				"err = %v, want rejected password change session error", err)
		})
	}
}

func TestLifecycleCurrentTokenVersionCacheMissReadsRepository(t *testing.T) {
	fixture := newLifecycleTestFixture(t)
	gomock.InOrder(
		fixture.tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
		fixture.users.EXPECT().GetTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(7), nil),
		fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), sessionTestUserID, int64(7)).Return(nil),
	)

	version, err := fixture.lifecycle.CurrentTokenVersion(context.Background(), sessionTestUserID)
	require.NoError(t, err,
		"CurrentTokenVersion: %v", err)
	require.EqualValues(t, 7, version,
		"version = %d, want 7", version)

}

func TestNewLifecycleRequiresLocalTokenVersionInvalidator(t *testing.T) {
	ctrl := gomock.NewController(t)
	var lifecycle Lifecycle
	require.NotPanics(t, func() {
		var err error
		lifecycle, err = NewLifecycle(
			NewMockUserTokenVersionStore(ctrl),
			NewMockTokenVersionCache(ctrl),
			NewMockRefreshSessionStore(ctrl),
			NewMockPasswordChangeSessionStore(ctrl),
			5,
			nil,
		)
		require.ErrorContains(t, err, "token version local invalidator is required")
	})
	require.Nil(t, lifecycle)
}

func TestLifecycleRevokeAllUserSessions(t *testing.T) {
	fixture := newLifecycleTestFixture(t)
	gomock.InOrder(
		fixture.users.EXPECT().IncrementTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(4), nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
		fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), sessionTestUserID, int64(4)).Return(nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
		fixture.sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), sessionTestUserID).Return(nil),
	)

	result, projectionErr, err := fixture.lifecycle.RevokeAllUserSessions(context.Background(), sessionTestUserID)
	require.NoError(t, err,
		"RevokeAllUserSessions: %v", err)
	require.NoError(t, projectionErr,
		"projection error = %v, want nil", projectionErr)
	require.False(t, result.UserID != sessionTestUserID || result.TokenVersion != 4,
		"result = %#v", result)

}

func TestLifecycleRevokeAllUserSessionsReturnsProjectionErrorAfterVersionIncrement(t *testing.T) {
	fixture := newLifecycleTestFixture(t)
	projectionErr := errors.New("redis unavailable")
	gomock.InOrder(
		fixture.users.EXPECT().IncrementTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(4), nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
		fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), sessionTestUserID, int64(4)).Return(projectionErr),
		fixture.tokenVersions.EXPECT().DeleteCachedTokenVersion(gomock.Any(), sessionTestUserID).Return(nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
		fixture.sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), sessionTestUserID).Return(nil),
	)

	result, gotProjectionErr, err := fixture.lifecycle.RevokeAllUserSessions(context.Background(), sessionTestUserID)

	require.NoError(t, err,
		"RevokeAllUserSessions err = %v, want nil", err)
	require.ErrorIs(t, gotProjectionErr, projectionErr,
		"projection err = %v, want cache error", gotProjectionErr)
	require.False(t, result == nil || result.UserID != sessionTestUserID || result.TokenVersion != 4,
		"result = %#v", result)
}

type lifecycleTestFixture struct {
	users           *MockUserTokenVersionStore
	tokenVersions   *MockTokenVersionCache
	sessions        *MockRefreshSessionStore
	passwordChanges *MockPasswordChangeSessionStore
	invalidator     *MockTokenVersionLocalInvalidator
	lifecycle       Lifecycle
}

func newLifecycleTestFixture(t *testing.T) *lifecycleTestFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	fixture := &lifecycleTestFixture{
		users:           NewMockUserTokenVersionStore(ctrl),
		tokenVersions:   NewMockTokenVersionCache(ctrl),
		sessions:        NewMockRefreshSessionStore(ctrl),
		passwordChanges: NewMockPasswordChangeSessionStore(ctrl),
		invalidator:     NewMockTokenVersionLocalInvalidator(ctrl),
	}
	var err error
	fixture.lifecycle, err = NewLifecycle(fixture.users, fixture.tokenVersions, fixture.sessions, fixture.passwordChanges, 5, fixture.invalidator)
	require.NoError(t, err)
	return fixture
}
