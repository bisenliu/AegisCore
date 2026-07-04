package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestLifecycleRotateTokenSessionMapsRejectedSession(t *testing.T) {
	for _, err := range []error{authdomain.ErrAuthSessionNotFound, authdomain.ErrAuthSessionMismatch} {
		t.Run(err.Error(), func(t *testing.T) {
			fixture := newLifecycleTestFixture(t, false)
			oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
			newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 2}
			fixture.sessions.EXPECT().
				RotateSession(gomock.Any(), oldSession, newSession, time.Hour, 5).
				Return(err)

			err := fixture.lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)
			require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
				"err = %v, want ErrTokenInvalid", err)

		})
	}
}

func TestLifecycleCurrentTokenVersionCacheMissReadsRepository(t *testing.T) {
	fixture := newLifecycleTestFixture(t, false)
	gomock.InOrder(
		fixture.tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), sessionTestUserID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
		fixture.users.EXPECT().GetTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(7), nil),
		fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), sessionTestUserID.String(), int64(7)).Return(nil),
	)

	version, err := fixture.lifecycle.CurrentTokenVersion(context.Background(), sessionTestUserID.String())
	require.NoError(t, err,
		"CurrentTokenVersion: %v", err)
	require.EqualValues(t, 7, version,
		"version = %d, want 7", version)

}

func TestLifecycleRevokeAllUserSessions(t *testing.T) {
	fixture := newLifecycleTestFixture(t, true)
	gomock.InOrder(
		fixture.users.EXPECT().IncrementTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(4), nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()).Return(nil),
		fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), sessionTestUserID.String(), int64(4)).Return(nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()).Return(nil),
		fixture.sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), sessionTestUserID.String()).Return(nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(sessionTestUserID.String()).Return(nil),
	)

	result, err := fixture.lifecycle.RevokeAllUserSessions(context.Background(), sessionTestUserID)
	require.NoError(t, err,
		"RevokeAllUserSessions: %v", err)
	require.False(t, result.UserID != sessionTestUserID || result.TokenVersion != 4,
		"result = %#v", result)
	require.NoError(t, result.ProjectionError,
		"projection error = %v, want nil", result.ProjectionError)

}

func TestLifecycleRevokeUserSessionsAtVersionReturnsLocalInvalidationErrors(t *testing.T) {
	fixture := newLifecycleTestFixture(t, true)
	userID := sessionTestUserID.String()
	invalidateErr := errors.New("local cache closed")
	gomock.InOrder(
		fixture.invalidator.EXPECT().InvalidateTokenVersion(userID).Return(invalidateErr),
		fixture.tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(4)).Return(nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(userID).Return(nil),
		fixture.sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), userID).Return(nil),
		fixture.invalidator.EXPECT().InvalidateTokenVersion(userID).Return(nil),
	)

	err := fixture.lifecycle.RevokeUserSessionsAtVersion(context.Background(), sessionTestUserID, 4)
	require.ErrorIs(t, err, invalidateErr,
		"err = %v, want local invalidation error", err)
	require.ErrorContains(t, err, "invalidate local token version cache before projection")
}

type lifecycleTestFixture struct {
	users         *MockUserTokenVersionStore
	tokenVersions *MockTokenVersionCache
	sessions      *MockRefreshSessionStore
	invalidator   *MockTokenVersionLocalInvalidator
	lifecycle     Lifecycle
}

func newLifecycleTestFixture(t *testing.T, withInvalidator bool) *lifecycleTestFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	fixture := &lifecycleTestFixture{
		users:         NewMockUserTokenVersionStore(ctrl),
		tokenVersions: NewMockTokenVersionCache(ctrl),
		sessions:      NewMockRefreshSessionStore(ctrl),
	}
	if withInvalidator {
		fixture.invalidator = NewMockTokenVersionLocalInvalidator(ctrl)
		fixture.lifecycle = NewLifecycle(fixture.users, fixture.tokenVersions, fixture.sessions, 5, fixture.invalidator)
		return fixture
	}
	fixture.lifecycle = NewLifecycle(fixture.users, fixture.tokenVersions, fixture.sessions, 5)
	return fixture
}
