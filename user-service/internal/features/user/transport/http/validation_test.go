package userhttp

import (
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestNormalizeCreateUser(t *testing.T) {
	t.Run("trims fields and lowercases username", func(t *testing.T) {
		req := CreateUserRequest{Nickname: " Alice ", Username: " Alice ", Password: " secret "}

		if err := NormalizeCreateUser(&req); err != nil {
			t.Fatalf("NormalizeCreateUser: %v", err)
		}
		if req.Nickname != "Alice" || req.Username != "alice" || req.Password != "secret" {
			t.Fatalf("req = %#v", req)
		}
	})

	t.Run("rejects blank user name", func(t *testing.T) {
		req := CreateUserRequest{Nickname: " ", Username: "alice", Password: "secret"}

		err := NormalizeCreateUser(&req)

		appErr := contracterrors.FromError(err)
		if appErr.Code != contracterrors.CodeValidationFailed || appErr.Message != messages.InvalidUsername {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("rejects blank password", func(t *testing.T) {
		req := CreateUserRequest{Nickname: "Alice", Username: "alice", Password: " "}

		err := NormalizeCreateUser(&req)

		appErr := contracterrors.FromError(err)
		if appErr.Code != contracterrors.CodeValidationFailed || appErr.Message != messages.InvalidPassword {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

func TestNormalizeListUsers(t *testing.T) {
	status := userdomain.UserStatusNormal
	req := ListUsersRequest{Cursor: " 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e ", PageSize: 20, Nickname: " Alice ", Username: " alice ", Status: &status}

	NormalizeListUsers(&req)

	if req.Cursor != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || req.PageSize != 20 || req.Limit != 20 || req.Nickname != "Alice" || req.Username != "alice" || req.Status == nil || *req.Status != status {
		t.Fatalf("req = %#v", req)
	}

	req = ListUsersRequest{PageSize: 0}
	NormalizeListUsers(&req)
	if req.PageSize != 10 || req.Limit != 10 {
		t.Fatalf("defaulted req = %#v", req)
	}

	req = ListUsersRequest{PageSize: 101}
	NormalizeListUsers(&req)
	if req.PageSize != 100 || req.Limit != 100 {
		t.Fatalf("capped req = %#v", req)
	}
}

func TestParseListCursor(t *testing.T) {
	cursor, err := ParseListCursor(ListUsersRequest{Cursor: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"})
	if err != nil || cursor == nil || cursor.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" {
		t.Fatalf("cursor=%v err=%v", cursor, err)
	}

	cursor, err = ParseListCursor(ListUsersRequest{})
	if err != nil || cursor != nil {
		t.Fatalf("empty cursor=%v err=%v", cursor, err)
	}

	_, err = ParseListCursor(ListUsersRequest{Cursor: "abc"})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeBadRequest || appErr.Message != messages.InvalidUserID {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestParseUserID(t *testing.T) {
	userID, err := ParseUserID(GetUserRequest{UserID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"})
	if err != nil || userID.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" {
		t.Fatalf("userID=%s err=%v", userID, err)
	}

	_, err = ParseUserID(GetUserRequest{UserID: "abc"})
	appErr := contracterrors.FromError(err)
	if appErr.Code != contracterrors.CodeBadRequest || appErr.Message != messages.InvalidUserID {
		t.Fatalf("err = %#v", appErr)
	}
}
