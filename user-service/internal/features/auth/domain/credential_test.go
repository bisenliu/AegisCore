package domain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestUserCredentialStatusRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                       string
		status                     identity.UserStatus
		wantCanLogin               bool
		wantRequiresPasswordChange bool
		wantCanChangePassword      bool
	}{
		{
			name:                       "normal user can login",
			status:                     identity.UserStatusNormal,
			wantCanLogin:               true,
			wantRequiresPasswordChange: false,
			wantCanChangePassword:      false,
		},
		{
			name:                       "disabled user is rejected",
			status:                     identity.UserStatusDisabled,
			wantCanLogin:               false,
			wantRequiresPasswordChange: false,
			wantCanChangePassword:      false,
		},
		{
			name:                       "must change password user can only change password",
			status:                     identity.UserStatusMustChangePassword,
			wantCanLogin:               false,
			wantRequiresPasswordChange: true,
			wantCanChangePassword:      true,
		},
		{
			name:                       "unknown status is rejected",
			status:                     identity.UserStatus(0),
			wantCanLogin:               false,
			wantRequiresPasswordChange: false,
			wantCanChangePassword:      false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			credential := UserCredential{Status: tc.status}
			require.Equal(t, tc.wantCanLogin, credential.CanLogin())
			require.Equal(t, tc.wantRequiresPasswordChange, credential.RequiresPasswordChange())
			require.Equal(t, tc.wantCanChangePassword, credential.CanChangePassword())
		})
	}
}
