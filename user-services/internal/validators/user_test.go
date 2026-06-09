package validators

import (
	"testing"

	"github.com/aegiscore/common/contract/response"
	userapi "github.com/aegiscore/user-services/internal/features/user/api"
	"github.com/aegiscore/user-services/internal/messages"
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
	status := userapi.UserStatusNormal
	req := userapi.ListUsersRequest{Page: 2, PageSize: 20, Nickname: " Alice ", Username: " alice ", Status: &status}

	NormalizeListUsers(&req)

	if req.Page != 2 || req.PageSize != 20 || req.Offset != 20 || req.Limit != 20 || req.Nickname != "Alice" || req.Username != "alice" || req.Status == nil || *req.Status != status {
		t.Fatalf("req = %#v", req)
	}

	req = userapi.ListUsersRequest{Page: -1, PageSize: 0}
	NormalizeListUsers(&req)
	if req.Page != 1 || req.PageSize != 10 || req.Offset != 0 || req.Limit != 10 {
		t.Fatalf("defaulted req = %#v", req)
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
