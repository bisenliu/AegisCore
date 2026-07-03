package validators

import (
	"testing"

	"github.com/stretchr/testify/require"

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
			require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
				"err = %v, want ErrInvalidCredentials", err)

		})
	}
}

func TestValidateRefreshTokenRejectsBlankToken(t *testing.T) {
	for _, token := range []string{"", " ", commonauth.TokenTypeBearer, commonauth.TokenPrefix} {
		err := ValidateRefreshToken(token)
		require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
			"token %q err = %v, want ErrTokenInvalid", token, err)

	}
}

func TestValidateChangePasswordCommandRejectsInvalidInput(t *testing.T) {
	{
		err := ValidateChangePasswordCommand("", "new-secret")
		require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
			"missing token err = %v, want ErrTokenInvalid", err)
	}
	{

		err := ValidateChangePasswordCommand("password-token", " ")
		require.ErrorIs(t, err, password.ErrEmptyPassword,
			"missing password err = %v, want ErrEmptyPassword", err)
	}

}
