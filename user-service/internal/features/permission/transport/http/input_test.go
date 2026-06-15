package permissionhttp

import (
	"testing"

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
	if err != nil {
		t.Fatalf("prepareListPermissionsQuery: %v", err)
	}
	if query.Cursor == nil || query.Cursor.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || query.PageSize != 10 || query.Limit != 10 || query.Module != "user" || query.HTTPMethod != "GET" {
		t.Fatalf("query = %#v", query)
	}

	_, err = prepareListPermissionsQuery(ListPermissionsRequest{Cursor: "bad"})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeBadRequest || appErr.Message != messages.InvalidPermission {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestPrepareUpdatePermissionCommand(t *testing.T) {
	cmd, err := prepareUpdatePermissionCommand(UpdatePermissionHTTPRequest{
		PermissionID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e",
		Name:         " List Users ",
		Description:  " Allows listing users ",
		Module:       " user ",
		HTTPMethod:   " GET ",
		PathTemplate: " /api/v1/users ",
		Active:       true,
	})
	if err != nil {
		t.Fatalf("prepareUpdatePermissionCommand: %v", err)
	}
	if cmd.PermissionID.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || cmd.Name != "List Users" || cmd.Description != "Allows listing users" || cmd.Module != "user" || cmd.HTTPMethod != "GET" || cmd.PathTemplate != "/api/v1/users" || !cmd.Active {
		t.Fatalf("cmd = %#v", cmd)
	}
}
