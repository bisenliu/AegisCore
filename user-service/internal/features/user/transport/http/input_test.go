package userhttp

import (
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestPrepareListUsersQuery(t *testing.T) {
	status := userdomain.UserStatusNormal
	query, err := prepareListUsersQuery(ListUsersRequest{
		Cursor:   " 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e ",
		PageSize: 101,
		Nickname: " Alice ",
		Username: " alice ",
		Status:   &status,
	})
	if err != nil {
		t.Fatalf("prepareListUsersQuery: %v", err)
	}
	if query.Cursor == nil || query.Cursor.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || query.PageSize != 100 || query.Limit != 100 || query.Nickname != "Alice" || query.Username != "alice" {
		t.Fatalf("query = %#v", query)
	}
	if query.Status == nil || *query.Status != status {
		t.Fatalf("status = %#v", query.Status)
	}

	_, err = prepareListUsersQuery(ListUsersRequest{Cursor: "bad"})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeBadRequest || appErr.Message != messages.InvalidUserID {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestPrepareCreateUserCommand(t *testing.T) {
	cmd, err := prepareCreateUserCommand(CreateUserRequest{Nickname: " Alice ", Username: " ALICE ", Password: " secret "})
	if err != nil {
		t.Fatalf("prepareCreateUserCommand: %v", err)
	}
	if cmd.Nickname != "Alice" || cmd.Username != "alice" || cmd.Password != "secret" {
		t.Fatalf("cmd = %#v", cmd)
	}

	_, err = prepareCreateUserCommand(CreateUserRequest{Nickname: "Alice", Username: "alice", Password: " "})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeValidationFailed || appErr.Message != messages.InvalidPassword {
		t.Fatalf("err = %#v", appErr)
	}
}
