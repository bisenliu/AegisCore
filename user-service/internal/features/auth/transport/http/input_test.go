package authhttp

import (
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestPrepareLoginCommand(t *testing.T) {
	cmd, err := prepareLoginCommand(LoginRequest{Username: " alice ", Password: " secret "})
	if err != nil {
		t.Fatalf("prepareLoginCommand: %v", err)
	}
	if cmd.Username != "alice" || cmd.Password != "secret" {
		t.Fatalf("cmd = %#v", cmd)
	}

	_, err = prepareLoginCommand(LoginRequest{Username: "alice", Password: " "})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeUnauthenticated || appErr.Message != messages.InvalidCredentials {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestPrepareTokenCommands(t *testing.T) {
	refresh, err := prepareRefreshTokenCommand(RefreshTokenRequest{RefreshToken: " " + commonauth.TokenPrefix + "refresh-token "})
	if err != nil {
		t.Fatalf("prepareRefreshTokenCommand: %v", err)
	}
	if refresh.RefreshToken != "refresh-token" {
		t.Fatalf("refresh = %#v", refresh)
	}

	change, err := prepareChangePasswordCommand(ChangePasswordRequest{Token: " " + commonauth.TokenPrefix + "password-token ", NewPassword: " new-secret "})
	if err != nil {
		t.Fatalf("prepareChangePasswordCommand: %v", err)
	}
	if change.Token != "password-token" || change.NewPassword != "new-secret" {
		t.Fatalf("change = %#v", change)
	}

	_, err = prepareRefreshTokenCommand(RefreshTokenRequest{RefreshToken: " " + commonauth.TokenPrefix})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeTokenInvalid || appErr.Message != messages.MissingSession {
		t.Fatalf("err = %#v", appErr)
	}
}
