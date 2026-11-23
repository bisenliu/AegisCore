package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	platformauth "github.com/aegiscore/platform/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestAuthUseCaseLogoutCurrentDeletesSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	expiresAt := time.Now().Add(10 * time.Minute)
	ctx := authenticatedLogoutContext("s-123", "jti-current", expiresAt)

	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID, "s-123").Return(nil)
	metrics.EXPECT().LogoutSucceeded(gomock.Any(), authapplication.MetricsOperationLogoutCurrent)

	result, err := fixture.LogoutCurrentSession(ctx)
	require.NoError(t, err,
		"LogoutCurrentSession: %v", err)
	require.False(t, result == nil || !result.LoggedOut,
		"result = %#v", result)
	require.Contains(t, fixture.denylist.denied, "jti-current")

}

func TestAuthUseCaseLogoutCurrentRecordsDeleteFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := authenticatedLogoutContext("s-123", "jti-current", time.Now().Add(time.Minute))
	deleteErr := errors.New("delete failed")

	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID, "s-123").Return(deleteErr)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonSessionDeleteFailed)

	_, err := fixture.LogoutCurrentSession(ctx)
	require.ErrorIs(t, err, deleteErr,
		"err = %v, want delete error", err)

}

func TestAuthUseCaseLogoutCurrentRetriesDenylistWhenSessionMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := authenticatedLogoutContext("s-123", "jti-current", time.Now().Add(time.Minute))
	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID, "s-123").Return(authdomain.ErrAuthSessionNotFound)
	metrics.EXPECT().LogoutSucceeded(gomock.Any(), authapplication.MetricsOperationLogoutCurrent)

	result, err := fixture.LogoutCurrentSession(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.LoggedOut)
	require.Contains(t, fixture.denylist.denied, "jti-current")
}

func TestAuthUseCaseLogoutCurrentDenylistFailureIsIncomplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := authenticatedLogoutContext("s-123", "jti-current", time.Now().Add(time.Minute))
	fixture.denylist.denyErr = errors.New("redis unavailable")
	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID, "s-123").Return(nil)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonAccessDenylistFailed)

	_, err := fixture.LogoutCurrentSession(ctx)
	require.ErrorIs(t, err, authdomain.ErrSessionRevocationIncomplete)
}

func TestAuthUseCaseLogoutCurrentSkipsDenylistWhenTokenAlreadyExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	// Token 过期时间在过去，ttl <= 0
	ctx := authenticatedLogoutContext("s-123", "jti-expired", time.Now().Add(-time.Minute))
	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID, "s-123").Return(nil)
	metrics.EXPECT().LogoutSucceeded(gomock.Any(), authapplication.MetricsOperationLogoutCurrent)

	result, err := fixture.LogoutCurrentSession(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.LoggedOut)
	// 已自然过期的 Token 不会写入 denylist
	require.NotContains(t, fixture.denylist.denied, "jti-expired")
}

func authenticatedLogoutContext(sessionID string, tokenID string, expiresAt time.Time) context.Context {
	ctx := platformauth.WithSessionID(platformauth.WithUserID(context.Background(), authTestUserID.String()), sessionID)
	return platformauth.WithAccessToken(ctx, platformauth.AccessToken{ID: tokenID, UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: 2, ExpiresAt: expiresAt})
}

func TestAuthUseCaseLogoutAllIncrementsVersionAndDeletesSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := platformauth.WithSessionID(platformauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
	revocation := &authdomain.SessionRevocationResult{UserID: authTestUserID, TokenVersion: 3}

	fixture.sessions.EXPECT().RevokeAllUserSessions(gomock.Any(), authTestUserID).Return(revocation, nil, nil)
	metrics.EXPECT().LogoutSucceeded(gomock.Any(), authapplication.MetricsOperationLogoutAll)

	result, err := fixture.LogoutAllSessions(ctx)
	require.NoError(t, err,
		"LogoutAll: %v", err)
	require.False(t, result == nil || !result.LoggedOut,
		"result=%#v", result)

}

func TestAuthUseCaseLogoutAllReturnsIncompleteWhenRevocationProjectionFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := platformauth.WithSessionID(platformauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
	projectionErr := errors.New("projection failed")
	revocation := &authdomain.SessionRevocationResult{UserID: authTestUserID, TokenVersion: 3}

	fixture.sessions.EXPECT().RevokeAllUserSessions(gomock.Any(), authTestUserID).Return(revocation, projectionErr, nil)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutAll, authapplication.MetricsReasonSessionRevokeFailed)

	result, err := fixture.LogoutAllSessions(ctx)
	require.ErrorIs(t, err, authdomain.ErrSessionRevocationIncomplete)
	require.ErrorIs(t, err, projectionErr)
	require.Nil(t, result)

}

func TestAuthUseCaseLogoutAllMapsIncrementUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := platformauth.WithSessionID(platformauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	fixture.sessions.EXPECT().RevokeAllUserSessions(gomock.Any(), authTestUserID).Return(nil, nil, identity.ErrUserNotFound)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutAll, authapplication.MetricsReasonSessionRevokeFailed)

	_, err := fixture.LogoutAllSessions(ctx)
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}
