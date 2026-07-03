package authhttp

import (
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestPrepareLoginCommand(t *testing.T) {
	cmd, err := prepareLoginCommand(LoginRequest{Username: " alice ", Password: " secret "})
	require.NoError(t, err,
		"prepareLoginCommand: %v", err)
	require.False(t, cmd.Username != "alice" || cmd.Password != "secret",
		"cmd = %#v", cmd)

	_, err = prepareLoginCommand(LoginRequest{Username: "alice", Password: " "})
	appErr := contracterrors.FromError(err)
	require.False(t, appErr.Code != contracterrors.CodeUnauthenticated || appErr.Message != messages.InvalidCredentials,
		"err = %#v", appErr)

}

func TestPrepareTokenCommands(t *testing.T) {
	refresh, err := prepareRefreshTokenCommand(RefreshTokenRequest{RefreshToken: " " + commonauth.TokenPrefix + "refresh-token "})
	require.NoError(t, err,
		"prepareRefreshTokenCommand: %v", err)
	require.Equal(t, "refresh-token", refresh.RefreshToken,
		"refresh = %#v", refresh)

	change, err := prepareChangePasswordCommand(ChangePasswordRequest{Token: " " + commonauth.TokenPrefix + "password-token ", NewPassword: " new-secret "})
	require.NoError(t, err,
		"prepareChangePasswordCommand: %v", err)
	require.False(t, change.Token != "password-token" || change.NewPassword != "new-secret",
		"change = %#v", change)

	_, err = prepareRefreshTokenCommand(RefreshTokenRequest{RefreshToken: " " + commonauth.TokenPrefix})
	appErr := contracterrors.FromError(err)
	require.False(t, appErr.Code != contracterrors.CodeTokenInvalid || appErr.Message != messages.MissingSession,
		"err = %#v", appErr)

}
