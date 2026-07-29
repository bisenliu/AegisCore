package schema

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestUserStatusDefaultMatchesDomainNormalStatus(t *testing.T) {
	var statusDefault any
	found := false
	for _, userField := range (User{}).Fields() {
		desc := userField.Descriptor()
		if desc.Name != "status" {
			continue
		}

		statusDefault = desc.Default
		found = true
		break
	}

	require.True(t, found)
	require.IsType(t, int64(0), statusDefault)
	require.Equal(t, int64(identity.UserStatusNormal), statusDefault)
}
