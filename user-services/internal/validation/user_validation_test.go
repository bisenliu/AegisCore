package validation

import (
	"testing"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/messages"
)

func TestNormalizeCreateUser(t *testing.T) {
	t.Run("trims fields and lowercases username", func(t *testing.T) {
		req := dto.CreateUserRequest{Nickname: " Alice ", Username: " Alice ", Password: " secret "}

		if err := NormalizeCreateUser(&req); err != nil {
			t.Fatalf("NormalizeCreateUser: %v", err)
		}
		if req.Nickname != "Alice" || req.Username != "alice" || req.Password != "secret" {
			t.Fatalf("req = %#v", req)
		}
	})

	t.Run("rejects blank user name", func(t *testing.T) {
		req := dto.CreateUserRequest{Nickname: " ", Username: "alice", Password: "secret"}

		err := NormalizeCreateUser(&req)

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != messages.InvalidUsername {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("rejects blank password", func(t *testing.T) {
		req := dto.CreateUserRequest{Nickname: "Alice", Username: "alice", Password: " "}

		err := NormalizeCreateUser(&req)

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != messages.InvalidPassword {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

func TestNormalizeListUsers(t *testing.T) {
	status := domain.UserStatusNormal
	req := dto.ListUsersRequest{Page: 2, PageSize: 20, Nickname: " Alice ", Username: " alice ", Status: &status}

	NormalizeListUsers(&req)

	if req.Page != 2 || req.PageSize != 20 || req.Offset != 20 || req.Limit != 20 || req.Nickname != "Alice" || req.Username != "alice" || req.Status == nil || *req.Status != status {
		t.Fatalf("req = %#v", req)
	}

	req = dto.ListUsersRequest{Page: -1, PageSize: 0}
	NormalizeListUsers(&req)
	if req.Page != 1 || req.PageSize != 10 || req.Offset != 0 || req.Limit != 10 {
		t.Fatalf("defaulted req = %#v", req)
	}
}

func TestParseUserID(t *testing.T) {
	userID, err := ParseUserID(dto.GetUserRequest{UserID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"})
	if err != nil || userID.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" {
		t.Fatalf("userID=%s err=%v", userID, err)
	}

	_, err = ParseUserID(dto.GetUserRequest{UserID: "abc"})
	appErr := response.FromError(err)
	if appErr.Code != response.CodeBadRequest || appErr.Message != messages.InvalidUserID {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestNormalizeLogin(t *testing.T) {
	req := dto.LoginRequest{Username: " alice ", Password: " secret "}
	if err := NormalizeLogin(&req); err != nil {
		t.Fatalf("NormalizeLogin: %v", err)
	}
	if req.Username != "alice" || req.Password != "secret" {
		t.Fatalf("req = %#v", req)
	}

	req = dto.LoginRequest{Username: "alice", Password: " "}
	err := NormalizeLogin(&req)
	appErr := response.FromError(err)
	if appErr.Code != response.CodeUnauthenticated || appErr.Message != messages.InvalidCredentials {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestNormalizeChangePassword(t *testing.T) {
	req := dto.ChangePasswordRequest{Token: " " + auth.TokenPrefix + "token ", NewPassword: " new-secret "}
	if err := NormalizeChangePassword(&req); err != nil {
		t.Fatalf("NormalizeChangePassword: %v", err)
	}
	if req.Token != "token" || req.NewPassword != "new-secret" {
		t.Fatalf("req = %#v", req)
	}

	req = dto.ChangePasswordRequest{Token: auth.TokenPrefix + "token", NewPassword: " "}
	err := NormalizeChangePassword(&req)
	appErr := response.FromError(err)
	if appErr.Code != response.CodeValidationFailed || appErr.Message != messages.InvalidPassword {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestNormalizeRefresh(t *testing.T) {
	req := dto.RefreshTokenRequest{RefreshToken: " " + auth.TokenPrefix + "refresh-token "}
	if err := NormalizeRefresh(&req); err != nil {
		t.Fatalf("NormalizeRefresh: %v", err)
	}
	if req.RefreshToken != "refresh-token" {
		t.Fatalf("req = %#v", req)
	}

	req = dto.RefreshTokenRequest{RefreshToken: " " + auth.TokenPrefix}
	err := NormalizeRefresh(&req)
	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid || appErr.Message != messages.MissingSession {
		t.Fatalf("err = %#v", appErr)
	}
}
