package sessions

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	platformauth "github.com/aegiscore/platform/security/auth"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
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

func TestLifecyclePersistsAndDeletesSessions(t *testing.T) {
	t.Run("create refresh session", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		fixture.sessions.EXPECT().
			CreateSession(gomock.Any(), gomock.Any(), time.Hour, 5).
			DoAndReturn(func(_ context.Context, session authdomain.AuthSession, _ time.Duration, _ int) error {
				require.Equal(t, sessionTestUserID, session.UserID)
				require.Equal(t, "session-1", session.SessionID)
				require.EqualValues(t, 3, session.TokenVersion)
				require.WithinDuration(t, time.Now().Add(time.Hour), session.ExpiresAt, 2*time.Second)
				return nil
			})

		require.NoError(t, fixture.lifecycle.CreateTokenSession(context.Background(), sessionTestUserID, "session-1", 3, time.Hour))
	})

	t.Run("create refresh session store failure", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		storeErr := errors.New("session store unavailable")
		fixture.sessions.EXPECT().CreateSession(gomock.Any(), gomock.Any(), time.Hour, 5).Return(storeErr)

		err := fixture.lifecycle.CreateTokenSession(context.Background(), sessionTestUserID, "session-1", 3, time.Hour)
		require.ErrorIs(t, err, storeErr)
	})

	t.Run("create password change session", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		fixture.passwordChanges.EXPECT().
			CreatePasswordChangeSession(gomock.Any(), gomock.Any(), 15*time.Minute).
			DoAndReturn(func(_ context.Context, session authdomain.PasswordChangeSession, _ time.Duration) error {
				require.Equal(t, sessionTestUserID, session.UserID)
				require.Equal(t, "password-session", session.SessionID)
				require.Equal(t, "password-token", session.TokenID)
				require.EqualValues(t, 2, session.TokenVersion)
				require.WithinDuration(t, time.Now().Add(15*time.Minute), session.ExpiresAt, 2*time.Second)
				return nil
			})

		require.NoError(t, fixture.lifecycle.CreatePasswordChangeSession(context.Background(), sessionTestUserID, "password-session", "password-token", 2, 15*time.Minute))
	})

	t.Run("create password change session store failure", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		storeErr := errors.New("password session store unavailable")
		fixture.passwordChanges.EXPECT().CreatePasswordChangeSession(gomock.Any(), gomock.Any(), 15*time.Minute).Return(storeErr)

		err := fixture.lifecycle.CreatePasswordChangeSession(context.Background(), sessionTestUserID, "password-session", "password-token", 2, 15*time.Minute)
		require.ErrorIs(t, err, storeErr)
	})

	t.Run("delete session", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		fixture.sessions.EXPECT().DeleteSession(gomock.Any(), sessionTestUserID, "session-1").Return(nil)

		require.NoError(t, fixture.lifecycle.DeleteSession(context.Background(), sessionTestUserID, "session-1"))
	})

	t.Run("delete session store failure", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		storeErr := errors.New("delete session failed")
		fixture.sessions.EXPECT().DeleteSession(gomock.Any(), sessionTestUserID, "session-1").Return(storeErr)

		err := fixture.lifecycle.DeleteSession(context.Background(), sessionTestUserID, "session-1")
		require.ErrorIs(t, err, storeErr)
	})
}

func TestLifecycleValidateRefreshSessionBoundaries(t *testing.T) {
	claims := &authtokens.Claims{UserID: sessionTestUserID, SessionID: "session-1", TokenVersion: 2}
	stored := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "session-1", TokenVersion: 2}

	t.Run("session missing maps to invalid token", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		fixture.sessions.EXPECT().GetSession(gomock.Any(), sessionTestUserID, "session-1").Return(authdomain.AuthSession{}, authdomain.ErrAuthSessionNotFound)

		_, version, err := fixture.lifecycle.ValidateRefreshSession(context.Background(), claims)
		require.ErrorIs(t, err, authdomain.ErrAuthSessionNotFound)
		require.ErrorIs(t, err, authdomain.ErrTokenInvalid)
		require.Zero(t, version)
	})

	t.Run("session store failure is not remapped", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		storeErr := errors.New("session store unavailable")
		fixture.sessions.EXPECT().GetSession(gomock.Any(), sessionTestUserID, "session-1").Return(authdomain.AuthSession{}, storeErr)

		_, _, err := fixture.lifecycle.ValidateRefreshSession(context.Background(), claims)
		require.ErrorIs(t, err, storeErr)
		require.NotErrorIs(t, err, authdomain.ErrTokenInvalid)
	})

	t.Run("claim mismatch stops before token version lookup", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		mismatched := stored
		mismatched.TokenVersion = 1
		fixture.sessions.EXPECT().GetSession(gomock.Any(), sessionTestUserID, "session-1").Return(mismatched, nil)

		_, _, err := fixture.lifecycle.ValidateRefreshSession(context.Background(), claims)
		require.ErrorIs(t, err, authdomain.ErrAuthSessionMismatch)
		require.ErrorIs(t, err, authdomain.ErrTokenInvalid)
	})

	t.Run("missing user is distinct from token invalid", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		gomock.InOrder(
			fixture.sessions.EXPECT().GetSession(gomock.Any(), sessionTestUserID, "session-1").Return(stored, nil),
			fixture.tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
			fixture.users.EXPECT().GetTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(0), identity.ErrUserNotFound),
		)

		_, _, err := fixture.lifecycle.ValidateRefreshSession(context.Background(), claims)
		require.ErrorIs(t, err, identity.ErrUserNotFound)
		require.NotErrorIs(t, err, authdomain.ErrTokenInvalid)
	})

	t.Run("token version lookup failure", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		lookupErr := errors.New("token version store unavailable")
		gomock.InOrder(
			fixture.sessions.EXPECT().GetSession(gomock.Any(), sessionTestUserID, "session-1").Return(stored, nil),
			fixture.tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(0), lookupErr),
		)

		_, _, err := fixture.lifecycle.ValidateRefreshSession(context.Background(), claims)
		require.ErrorIs(t, err, lookupErr)
	})

	t.Run("revoked token version", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		gomock.InOrder(
			fixture.sessions.EXPECT().GetSession(gomock.Any(), sessionTestUserID, "session-1").Return(stored, nil),
			fixture.tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(3), nil),
		)

		_, _, err := fixture.lifecycle.ValidateRefreshSession(context.Background(), claims)
		require.ErrorIs(t, err, authdomain.ErrTokenInvalid)
		require.ErrorIs(t, err, platformauth.ErrTokenVersionMismatch)
	})

	t.Run("matching session and version", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		gomock.InOrder(
			fixture.sessions.EXPECT().GetSession(gomock.Any(), sessionTestUserID, "session-1").Return(stored, nil),
			fixture.tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(2), nil),
		)

		session, version, err := fixture.lifecycle.ValidateRefreshSession(context.Background(), claims)
		require.NoError(t, err)
		require.Equal(t, stored, session)
		require.EqualValues(t, 2, version)
	})
}

func TestLifecycleRotateAndConsumeUnexpectedStoreErrors(t *testing.T) {
	t.Run("rotate store failure is not remapped", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		storeErr := errors.New("rotate store unavailable")
		oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-old", TokenVersion: 2}
		newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 2}
		fixture.sessions.EXPECT().RotateSession(gomock.Any(), oldSession, newSession, time.Hour, 5).Return(storeErr)

		err := fixture.lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)
		require.ErrorIs(t, err, storeErr)
		require.NotErrorIs(t, err, authdomain.ErrTokenInvalid)
	})

	t.Run("rotate success", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-old", TokenVersion: 2}
		newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 2}
		fixture.sessions.EXPECT().RotateSession(gomock.Any(), oldSession, newSession, time.Hour, 5).Return(nil)

		require.NoError(t, fixture.lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour))
	})

	t.Run("consume store failure is not remapped", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		storeErr := errors.New("consume store unavailable")
		claims := &authtokens.Claims{UserID: sessionTestUserID, SessionID: "password-session", TokenVersion: 2, RegisteredClaims: jwtv5.RegisteredClaims{ID: "password-token"}}
		fixture.passwordChanges.EXPECT().ConsumePasswordChangeSession(gomock.Any(), gomock.Any()).Return(storeErr)

		err := fixture.lifecycle.ConsumePasswordChangeClaims(context.Background(), claims)
		require.ErrorIs(t, err, storeErr)
		require.NotErrorIs(t, err, authdomain.ErrTokenInvalid)
	})

	t.Run("consume success", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		claims := &authtokens.Claims{UserID: sessionTestUserID, SessionID: "password-session", TokenVersion: 2, RegisteredClaims: jwtv5.RegisteredClaims{ID: "password-token"}}
		fixture.passwordChanges.EXPECT().
			ConsumePasswordChangeSession(gomock.Any(), authdomain.PasswordChangeSession{UserID: claims.UserID, SessionID: claims.SessionID, TokenID: claims.ID, TokenVersion: claims.TokenVersion}).
			Return(nil)

		require.NoError(t, fixture.lifecycle.ConsumePasswordChangeClaims(context.Background(), claims))
	})
}

func TestLifecycleRevokeBoundaries(t *testing.T) {
	t.Run("missing user does not refresh projection", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		fixture.users.EXPECT().IncrementTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(0), identity.ErrUserNotFound)

		result, projectionErr, err := fixture.lifecycle.RevokeAllUserSessions(context.Background(), sessionTestUserID)
		require.Nil(t, result)
		require.NoError(t, projectionErr)
		require.ErrorIs(t, err, identity.ErrUserNotFound)
	})

	t.Run("increment failure is not treated as missing user", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		storeErr := errors.New("version store unavailable")
		fixture.users.EXPECT().IncrementTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(0), storeErr)

		result, projectionErr, err := fixture.lifecycle.RevokeAllUserSessions(context.Background(), sessionTestUserID)
		require.Nil(t, result)
		require.NoError(t, projectionErr)
		require.ErrorIs(t, err, storeErr)
	})

	t.Run("cache refresh and eviction both fail", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		cacheErr := errors.New("cache write failed")
		evictErr := errors.New("cache delete failed")
		deleteErr := errors.New("session delete failed")
		gomock.InOrder(
			fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
			fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), sessionTestUserID, int64(8)).Return(cacheErr),
			fixture.tokenVersions.EXPECT().DeleteCachedTokenVersion(gomock.Any(), sessionTestUserID).Return(evictErr),
			fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
			fixture.sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), sessionTestUserID).Return(deleteErr),
		)

		err := fixture.lifecycle.RevokeUserSessionsAtVersion(context.Background(), sessionTestUserID, 8)
		require.ErrorIs(t, err, cacheErr)
		require.ErrorIs(t, err, evictErr)
		require.ErrorIs(t, err, deleteErr)
		require.ErrorContains(t, err, "refresh token version cache")
		require.ErrorContains(t, err, "delete token version cache")
		require.ErrorContains(t, err, "delete all user sessions")
	})

	t.Run("session cleanup failure after successful cache refresh", func(t *testing.T) {
		fixture := newLifecycleTestFixture(t)
		deleteErr := errors.New("session delete failed")
		gomock.InOrder(
			fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
			fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), sessionTestUserID, int64(8)).Return(nil),
			fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()),
			fixture.sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), sessionTestUserID).Return(deleteErr),
		)

		err := fixture.lifecycle.RevokeUserSessionsAtVersion(context.Background(), sessionTestUserID, 8)
		require.ErrorIs(t, err, deleteErr)
		require.ErrorContains(t, err, "delete all user sessions")
	})
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
