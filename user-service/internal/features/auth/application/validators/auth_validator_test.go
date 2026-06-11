package validators

import (
	"errors"
	"testing"

	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestValidateLoginCommandRejectsBlankFields(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "blank username", username: " ", password: "secret"},
		{name: "blank password", username: "alice", password: " "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLoginCommand(tt.username, tt.password)

			if !errors.Is(err, authdomain.ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestValidateRefreshTokenRejectsBlankToken(t *testing.T) {
	for _, token := range []string{"", " ", commonauth.TokenTypeBearer, commonauth.TokenPrefix} {
		err := ValidateRefreshToken(token)

		if !errors.Is(err, authdomain.ErrTokenInvalid) {
			t.Fatalf("token %q err = %v, want ErrTokenInvalid", token, err)
		}
	}
}

func TestValidateChangePasswordCommandRejectsInvalidInput(t *testing.T) {
	if err := ValidateChangePasswordCommand("", "new-secret"); !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("missing token err = %v, want ErrTokenInvalid", err)
	}
	if err := ValidateChangePasswordCommand("password-token", " "); !errors.Is(err, password.ErrEmptyPassword) {
		t.Fatalf("missing password err = %v, want ErrEmptyPassword", err)
	}
}
