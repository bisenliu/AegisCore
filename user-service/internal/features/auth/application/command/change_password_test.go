package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestAuthUseCaseChangePassword(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	gomock.InOrder(
		fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil),
		fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil),
		fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(&authdomain.CredentialUpdateResult{UserID: authTestUserID, TokenVersion: 3}, nil),
		fixture.sessions.EXPECT().RevokeUserSessionsAtVersion(gomock.Any(), authTestUserID, int64(3)).Return(nil),
	)

	result, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})
	require.NoError(t, err,
		"ChangePassword: %v", err)
	require.False(t, result == nil || !result.Changed,
		"result=%#v", result)

}

func TestAuthUseCaseChangePasswordIncrementsTokenVersionOnce(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil)
	fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(&authdomain.CredentialUpdateResult{UserID: authTestUserID, TokenVersion: 3}, nil).Times(1)
	fixture.sessions.EXPECT().RevokeUserSessionsAtVersion(gomock.Any(), authTestUserID, int64(3)).Return(nil).Times(1)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})
	require.NoError(t, err,
		"ChangePassword: %v", err)

}

func TestAuthUseCaseChangePasswordSucceedsWhenRevocationProjectionFails(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil)
	fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(&authdomain.CredentialUpdateResult{UserID: authTestUserID, TokenVersion: 3}, nil)
	fixture.sessions.EXPECT().RevokeUserSessionsAtVersion(gomock.Any(), authTestUserID, int64(3)).Return(errors.New("projection failed"))

	result, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})
	require.NoError(t, err,
		"ChangePassword: %v", err)
	require.False(t, result == nil || !result.Changed,
		"result = %#v", result)

}

func TestAuthUseCaseChangePasswordMapsCredentialUpdateNotFound(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(nil)
	fixture.credentials.EXPECT().ChangePassword(gomock.Any(), authTestUserID, "new-secret").Return(nil, identity.ErrUserNotFound)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}

func TestAuthUseCaseChangePasswordMapsTokenVersionUserNotFound(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	claims := passwordChangeClaims("pc-123", 2)

	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "password-change").Return(claims, authTestUserID, nil)
	fixture.sessions.EXPECT().ValidatePasswordChangeClaims(gomock.Any(), claims).Return(identity.ErrUserNotFound)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "password-change", NewPassword: "new-secret"})
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}

func TestAuthUseCaseChangePasswordRejectsAccessToken(t *testing.T) {
	fixture := newAuthCommandFixture(t, defaultAuthConfig(true), nil)
	fixture.tokens.EXPECT().ParsePasswordChangeToken(gomock.Any(), "access").Return(nil, uuid.Nil, authdomain.ErrTokenInvalid)

	_, err := fixture.ChangePassword(context.Background(), ChangePasswordCommand{Token: "access", NewPassword: "new-secret"})
	require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
		"err = %v, want authdomain.ErrTokenInvalid", err)

}
