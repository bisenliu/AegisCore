package validators

import (
	"testing"

	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestCreateUserStatus(t *testing.T) {
	if got := CreateUserStatus(nil); got != identity.UserStatusNormal {
		t.Fatalf("CreateUserStatus(nil) = %d, want %d", got, identity.UserStatusNormal)
	}

	status := identity.UserStatusDisabled
	if got := CreateUserStatus(&status); got != identity.UserStatusDisabled {
		t.Fatalf("CreateUserStatus(disabled) = %d, want %d", got, identity.UserStatusDisabled)
	}
}
