package rolehttp

import (
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestPrepareListRolesQuery(t *testing.T) {
	query, err := prepareListRolesQuery(ListRolesRequest{Cursor: " 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e ", PageSize: 101})
	if err != nil {
		t.Fatalf("prepareListRolesQuery: %v", err)
	}
	if query.Cursor == nil || query.Cursor.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || query.PageSize != 100 || query.Limit != 100 {
		t.Fatalf("query = %#v", query)
	}

	_, err = prepareListRolesQuery(ListRolesRequest{Cursor: "bad"})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeBadRequest || appErr.Message != messages.InvalidRole {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestPrepareRoleBindingCommands(t *testing.T) {
	replaceUserRoles, err := prepareReplaceUserRolesCommand(ReplaceUserRolesHTTPRequest{
		UserID:  "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e",
		RoleIDs: []string{" 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f "},
	})
	if err != nil {
		t.Fatalf("prepareReplaceUserRolesCommand: %v", err)
	}
	if replaceUserRoles.UserID.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || len(replaceUserRoles.RoleIDs) != 1 || replaceUserRoles.RoleIDs[0].String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f" {
		t.Fatalf("replaceUserRoles = %#v", replaceUserRoles)
	}

	rolePermission, err := prepareRolePermissionCommand(RolePermissionHTTPRequest{
		RoleID:       "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f",
		PermissionID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d50",
	})
	if err != nil {
		t.Fatalf("prepareRolePermissionCommand: %v", err)
	}
	if rolePermission.RoleID.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f" || rolePermission.PermissionID.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d50" {
		t.Fatalf("rolePermission = %#v", rolePermission)
	}

	_, err = prepareReplaceRolePermissionsCommand(ReplaceRolePermissionsHTTPRequest{
		RoleID:        "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f",
		PermissionIDs: []string{"bad"},
	})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeBadRequest || appErr.Message != messages.InvalidPermission {
		t.Fatalf("err = %#v", appErr)
	}
}
