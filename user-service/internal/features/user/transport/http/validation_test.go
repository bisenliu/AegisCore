package userhttp

import (
	"testing"

	"github.com/aegiscore/common/contract/response"
	userapi "github.com/aegiscore/user-service/internal/features/user/api"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestNormalizeCreateUser(t *testing.T) {
	t.Run("trims fields and lowercases username", func(t *testing.T) {
		req := userapi.CreateUserRequest{Nickname: " Alice ", Username: " Alice ", Password: " secret "}

		if err := NormalizeCreateUser(&req); err != nil {
			t.Fatalf("NormalizeCreateUser: %v", err)
		}
		if req.Nickname != "Alice" || req.Username != "alice" || req.Password != "secret" {
			t.Fatalf("req = %#v", req)
		}
	})

	t.Run("rejects blank user name", func(t *testing.T) {
		req := userapi.CreateUserRequest{Nickname: " ", Username: "alice", Password: "secret"}

		err := NormalizeCreateUser(&req)

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != messages.InvalidUsername {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("rejects blank password", func(t *testing.T) {
		req := userapi.CreateUserRequest{Nickname: "Alice", Username: "alice", Password: " "}

		err := NormalizeCreateUser(&req)

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != messages.InvalidPassword {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

func TestNormalizeListUsers(t *testing.T) {
	status := userdomain.UserStatusNormal
	req := userapi.ListUsersRequest{Cursor: " 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e ", PageSize: 20, Nickname: " Alice ", Username: " alice ", Status: &status}

	NormalizeListUsers(&req)

	if req.Cursor != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || req.PageSize != 20 || req.Limit != 20 || req.Nickname != "Alice" || req.Username != "alice" || req.Status == nil || *req.Status != status {
		t.Fatalf("req = %#v", req)
	}

	req = userapi.ListUsersRequest{PageSize: 0}
	NormalizeListUsers(&req)
	if req.PageSize != 10 || req.Limit != 10 {
		t.Fatalf("defaulted req = %#v", req)
	}

	req = userapi.ListUsersRequest{PageSize: 101}
	NormalizeListUsers(&req)
	if req.PageSize != 100 || req.Limit != 100 {
		t.Fatalf("capped req = %#v", req)
	}
}

func TestParseListCursor(t *testing.T) {
	cursor, err := ParseListCursor(userapi.ListUsersRequest{Cursor: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"})
	if err != nil || cursor == nil || cursor.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" {
		t.Fatalf("cursor=%v err=%v", cursor, err)
	}

	cursor, err = ParseListCursor(userapi.ListUsersRequest{})
	if err != nil || cursor != nil {
		t.Fatalf("empty cursor=%v err=%v", cursor, err)
	}

	_, err = ParseListCursor(userapi.ListUsersRequest{Cursor: "abc"})
	appErr := response.FromError(err)
	if appErr.Code != response.CodeBadRequest || appErr.Message != messages.InvalidUserID {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestParseUserID(t *testing.T) {
	userID, err := ParseUserID(userapi.GetUserRequest{UserID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"})
	if err != nil || userID.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" {
		t.Fatalf("userID=%s err=%v", userID, err)
	}

	_, err = ParseUserID(userapi.GetUserRequest{UserID: "abc"})
	appErr := response.FromError(err)
	if appErr.Code != response.CodeBadRequest || appErr.Message != messages.InvalidUserID {
		t.Fatalf("err = %#v", appErr)
	}
}
