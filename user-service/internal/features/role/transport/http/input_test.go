package rolehttp

import (
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestPrepareListRolesQuery(t *testing.T) {
	query, err := prepareListRolesQuery(ListRolesRequest{Cursor: " 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e ", PageSize: 101})
	require.NoError(t, err)
	require.NotNil(t, query.Cursor)
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", query.Cursor.String())
	require.Equal(t, 100, query.PageSize)
	require.Equal(t, 100, query.Limit)

	_, err = prepareListRolesQuery(ListRolesRequest{Cursor: "bad"})
	require.Error(t, err)
	appErr := contracterrors.FromError(err)
	require.NotNil(t, appErr)
	require.Equal(t, contracterrors.CodeBadRequest, appErr.Code)
	require.Equal(t, messages.InvalidRole, appErr.Message)
}

func TestPrepareRoleBindingCommands(t *testing.T) {
	replaceUserRoles, err := prepareReplaceUserRolesCommand(ReplaceUserRolesHTTPRequest{
		UserID:  "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e",
		RoleIDs: []string{" 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f "},
	})
	require.NoError(t, err)
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", replaceUserRoles.UserID.String())
	require.Len(t, replaceUserRoles.RoleIDs, 1)
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f", replaceUserRoles.RoleIDs[0].String())

	rolePermission, err := prepareRolePermissionCommand(RolePermissionHTTPRequest{
		RoleID:       "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f",
		PermissionID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d50",
	})
	require.NoError(t, err)
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f", rolePermission.RoleID.String())
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d50", rolePermission.PermissionID.String())

	_, err = prepareReplaceRolePermissionsCommand(ReplaceRolePermissionsHTTPRequest{
		RoleID:        "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f",
		PermissionIDs: []string{"bad"},
	})
	require.Error(t, err)
	appErr := contracterrors.FromError(err)
	require.NotNil(t, appErr)
	require.Equal(t, contracterrors.CodeBadRequest, appErr.Code)
	require.Equal(t, messages.InvalidPermission, appErr.Message)
}
