package domain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestUserStateRules(t *testing.T) {
	tests := []struct {
		name                  string
		status                identity.UserStatus
		wantCanLogin          bool
		wantPasswordChange    bool
		wantCanChangePassword bool
	}{
		{name: "normal can login", status: identity.UserStatusNormal, wantCanLogin: true},
		{name: "disabled cannot login", status: identity.UserStatusDisabled},
		{name: "must change password cannot login", status: identity.UserStatusMustChangePassword, wantPasswordChange: true, wantCanChangePassword: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{Status: tt.status}
			require.Equal(t, tt.wantCanLogin, user.CanLogin())
			require.Equal(t, tt.wantPasswordChange, user.RequiresPasswordChange())
			require.Equal(t, tt.wantCanChangePassword, user.CanChangePassword())
		})
	}
}
