package redis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyCatalogBuildsRBACPolicyKeys(t *testing.T) {
	catalog, err := NewKeyCatalog(" aegiscore-user-service ")
	require.NoError(t, err)

	require.Equal(t, "aegiscore-user-service:rbac:policy:{sync}:version", catalog.PolicyVersionKey())
	require.Equal(t, "aegiscore-user-service:rbac:policy:{sync}:refresh", catalog.PolicyChannel())
}

func mustKeyCatalog(appName string) KeyCatalog {
	catalog, err := NewKeyCatalog(appName)
	if err != nil {
		panic(err)
	}
	return catalog
}
