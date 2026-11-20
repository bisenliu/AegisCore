package authhttp

import (
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/platform/contract/errors"
	platformauth "github.com/aegiscore/platform/security/auth"
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
	refresh, err := prepareRefreshTokenCommand(RefreshTokenRequest{RefreshToken: " " + platformauth.TokenPrefix + "refresh-token "})
	require.NoError(t, err,
		"prepareRefreshTokenCommand: %v", err)
	require.Equal(t, "refresh-token", refresh.RefreshToken,
		"refresh = %#v", refresh)

	forceChange, err := prepareForceChangePasswordCommand(ForceChangePasswordRequest{Token: " " + platformauth.TokenPrefix + "password-token ", NewPassword: " new-secret-123 "})
	require.NoError(t, err,
		"prepareForceChangePasswordCommand: %v", err)
	require.False(t, forceChange.Token != "password-token" || forceChange.NewPassword != "new-secret-123",
		"forceChange = %#v", forceChange)

	_, err = prepareRefreshTokenCommand(RefreshTokenRequest{RefreshToken: " " + platformauth.TokenPrefix})
	appErr := contracterrors.FromError(err)
	require.False(t, appErr.Code != contracterrors.CodeTokenInvalid || appErr.Message != messages.MissingSession,
		"err = %#v", appErr)

}
