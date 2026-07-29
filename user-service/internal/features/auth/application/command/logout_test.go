package command

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestAuthUseCaseLogoutCurrentDeletesSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID, "s-123").Return(nil)
	metrics.EXPECT().LogoutSucceeded(gomock.Any(), authapplication.MetricsOperationLogoutCurrent)

	result, err := fixture.LogoutCurrentSession(ctx)
	require.NoError(t, err,
		"LogoutCurrentSession: %v", err)
	require.False(t, result == nil || !result.LoggedOut,
		"result = %#v", result)

}

func TestAuthUseCaseLogoutCurrentRecordsDeleteFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
	deleteErr := errors.New("delete failed")

	fixture.sessions.EXPECT().DeleteSession(gomock.Any(), authTestUserID, "s-123").Return(deleteErr)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonSessionDeleteFailed)

	_, err := fixture.LogoutCurrentSession(ctx)
	require.ErrorIs(t, err, deleteErr,
		"err = %v, want delete error", err)

}

func TestAuthUseCaseLogoutAllIncrementsVersionAndDeletesSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	fixture := newAuthCommandFixtureWithController(ctrl, defaultAuthConfig(true), metrics)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
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
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")
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
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	fixture.sessions.EXPECT().RevokeAllUserSessions(gomock.Any(), authTestUserID).Return(nil, nil, identity.ErrUserNotFound)
	metrics.EXPECT().LogoutFailed(gomock.Any(), authapplication.MetricsOperationLogoutAll, authapplication.MetricsReasonSessionRevokeFailed)

	_, err := fixture.LogoutAllSessions(ctx)
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}
