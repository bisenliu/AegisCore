package permissionhttp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareListPermissionsQuery(t *testing.T) {
	query, err := prepareListPermissionsQuery(ListPermissionsRequest{
		Module:     " user ",
		HTTPMethod: " GET ",
	})
	require.NoError(t, err)
	require.Equal(t, "user", query.Module)
	require.Equal(t, "GET", query.HTTPMethod)
}
