package permissionhttp

import (
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestPrepareListPermissionsQuery(t *testing.T) {
	query, err := prepareListPermissionsQuery(ListPermissionsRequest{
		Cursor:     " 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e ",
		PageSize:   0,
		Module:     " user ",
		HTTPMethod: " GET ",
	})
	require.NoError(t, err)
	require.NotNil(t, query.Cursor)
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", query.Cursor.String())
	require.Equal(t, 10, query.PageSize)
	require.Equal(t, 10, query.Limit)
	require.Equal(t, "user", query.Module)
	require.Equal(t, "GET", query.HTTPMethod)

	_, err = prepareListPermissionsQuery(ListPermissionsRequest{Cursor: "bad"})
	appErr := contracterrors.FromError(err)
	require.Equal(t, contracterrors.CodeBadRequest, appErr.Code)
	require.Equal(t, messages.InvalidPermission, appErr.Message)
}
