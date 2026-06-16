package identity

import (
	"encoding/json"
	"reflect"
	"testing"
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
			if got := tt.status.IsValid(); got != tt.want {
				t.Fatalf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserStatusAllowedValues(t *testing.T) {
	got := UserStatusNormal.AllowedValues()
	want := []string{"100", "200", "300"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllowedValues() = %#v, want %#v", got, want)
	}
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
			if got := tt.status.CanLogin(); got != tt.wantCanLogin {
				t.Fatalf("CanLogin() = %v, want %v", got, tt.wantCanLogin)
			}
			if got := tt.status.RequiresPasswordChange(); got != tt.wantRequiresChangePass {
				t.Fatalf("RequiresPasswordChange() = %v, want %v", got, tt.wantRequiresChangePass)
			}
		})
	}
}

func TestUserStatusUnmarshalText(t *testing.T) {
	var status UserStatus
	if err := status.UnmarshalText([]byte("300")); err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}
	if status != UserStatusMustChangePassword {
		t.Fatalf("status = %d, want %d", status, UserStatusMustChangePassword)
	}
}

func TestUserStatusUnmarshalJSON(t *testing.T) {
	var status UserStatus
	if err := json.Unmarshal([]byte("200"), &status); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	if status != UserStatusDisabled {
		t.Fatalf("status = %d, want %d", status, UserStatusDisabled)
	}
}

func TestUserStatusRejectsInvalidText(t *testing.T) {
	var status UserStatus
	if err := status.UnmarshalText([]byte("invalid")); err == nil {
		t.Fatal("UnmarshalText() error = nil, want error")
	}
}
