package user

import "testing"

func TestUserStateRules(t *testing.T) {
	tests := []struct {
		name                  string
		status                UserStatus
		wantCanLogin          bool
		wantPasswordChange    bool
		wantCanChangePassword bool
	}{
		{name: "normal can login", status: UserStatusNormal, wantCanLogin: true},
		{name: "disabled cannot login", status: UserStatusDisabled},
		{name: "must change password cannot login", status: UserStatusMustChangePassword, wantPasswordChange: true, wantCanChangePassword: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{Status: tt.status}
			if user.CanLogin() != tt.wantCanLogin {
				t.Fatalf("CanLogin() = %v, want %v", user.CanLogin(), tt.wantCanLogin)
			}
			if user.RequiresPasswordChange() != tt.wantPasswordChange {
				t.Fatalf("RequiresPasswordChange() = %v, want %v", user.RequiresPasswordChange(), tt.wantPasswordChange)
			}
			if user.CanChangePassword() != tt.wantCanChangePassword {
				t.Fatalf("CanChangePassword() = %v, want %v", user.CanChangePassword(), tt.wantCanChangePassword)
			}
		})
	}
}
