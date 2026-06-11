package validators

import (
	"testing"

	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
)

func TestCreateUserStatus(t *testing.T) {
	if got := CreateUserStatus(nil); got != userdomain.UserStatusNormal {
		t.Fatalf("CreateUserStatus(nil) = %d, want %d", got, userdomain.UserStatusNormal)
	}

	status := userdomain.UserStatusDisabled
	if got := CreateUserStatus(&status); got != userdomain.UserStatusDisabled {
		t.Fatalf("CreateUserStatus(disabled) = %d, want %d", got, userdomain.UserStatusDisabled)
	}
}
