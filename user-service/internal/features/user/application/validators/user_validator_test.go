package validators

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestCreateUserStatus(t *testing.T) {
	require.Equal(t, identity.UserStatusNormal, CreateUserStatus(nil))

	status := identity.UserStatusDisabled
	require.Equal(t, identity.UserStatusDisabled, CreateUserStatus(&status))
}
