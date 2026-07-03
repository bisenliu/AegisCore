package identity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserStatusIsValid(t *testing.T) {
	tests := []struct {
		name   string
		status UserStatus
		want   bool
	}{
		{name: "normal", status: UserStatusNormal, want: true},
		{name: "disabled", status: UserStatusDisabled, want: true},
		{name: "must change password", status: UserStatusMustChangePassword, want: true},
		{name: "unknown", status: UserStatus(999), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.status.IsValid())
		})
	}
}

func TestUserStatusAllowedValues(t *testing.T) {
	got := UserStatusNormal.AllowedValues()
	want := []string{"100", "200", "300"}
	require.Equal(t, want, got)
}

func TestUserStatusLifecycleRules(t *testing.T) {
	tests := []struct {
		name                   string
		status                 UserStatus
		wantCanLogin           bool
		wantRequiresChangePass bool
	}{
		{name: "normal", status: UserStatusNormal, wantCanLogin: true},
		{name: "disabled", status: UserStatusDisabled},
		{name: "must change password", status: UserStatusMustChangePassword, wantRequiresChangePass: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantCanLogin, tt.status.CanLogin())
			require.Equal(t, tt.wantRequiresChangePass, tt.status.RequiresPasswordChange())
		})
	}
}

func TestUserStatusUnmarshalText(t *testing.T) {
	var status UserStatus
	require.NoError(t, status.UnmarshalText([]byte("300")))
	require.Equal(t, UserStatusMustChangePassword, status)
}

func TestUserStatusUnmarshalJSON(t *testing.T) {
	var status UserStatus
	require.NoError(t, json.Unmarshal([]byte("200"), &status))
	require.Equal(t, UserStatusDisabled, status)
}

func TestUserStatusRejectsInvalidText(t *testing.T) {
	var status UserStatus
	require.Error(t, status.UnmarshalText([]byte("invalid")))
}
