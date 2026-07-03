package redis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyCatalogBuildsRBACPolicyKeys(t *testing.T) {
	catalog, err := NewKeyCatalog(" aegiscore-user-services ")
	require.NoError(t, err)

	require.Equal(t, "aegiscore-user-services:rbac:policy:version", catalog.PolicyVersionKey())
	require.Equal(t, "aegiscore-user-services:rbac:policy:refresh", catalog.PolicyChannel())
}
