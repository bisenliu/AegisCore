package validators

import (
	"testing"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/api/auth"
	"github.com/aegiscore/user-services/internal/messages"
)

func TestNormalizeLogin(t *testing.T) {
	req := authapi.LoginRequest{Username: " alice ", Password: " secret "}
	if err := NormalizeLogin(&req); err != nil {
		t.Fatalf("NormalizeLogin: %v", err)
	}
	if req.Username != "alice" || req.Password != "secret" {
		t.Fatalf("req = %#v", req)
	}

	req = authapi.LoginRequest{Username: "alice", Password: " "}
	err := NormalizeLogin(&req)
	appErr := response.FromError(err)
	if appErr.Code != response.CodeUnauthenticated || appErr.Message != messages.InvalidCredentials {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestNormalizeChangePassword(t *testing.T) {
	req := authapi.ChangePasswordRequest{Token: " " + auth.TokenPrefix + "token ", NewPassword: " new-secret "}
	if err := NormalizeChangePassword(&req); err != nil {
		t.Fatalf("NormalizeChangePassword: %v", err)
	}
	if req.Token != "token" || req.NewPassword != "new-secret" {
		t.Fatalf("req = %#v", req)
	}

	req = authapi.ChangePasswordRequest{Token: auth.TokenPrefix + "token", NewPassword: " "}
	err := NormalizeChangePassword(&req)
	appErr := response.FromError(err)
	if appErr.Code != response.CodeValidationFailed || appErr.Message != messages.InvalidPassword {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestNormalizeRefresh(t *testing.T) {
	req := authapi.RefreshTokenRequest{RefreshToken: " " + auth.TokenPrefix + "refresh-token "}
	if err := NormalizeRefresh(&req); err != nil {
		t.Fatalf("NormalizeRefresh: %v", err)
	}
	if req.RefreshToken != "refresh-token" {
		t.Fatalf("req = %#v", req)
	}

	req = authapi.RefreshTokenRequest{RefreshToken: " " + auth.TokenPrefix}
	err := NormalizeRefresh(&req)
	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid || appErr.Message != messages.MissingSession {
		t.Fatalf("err = %#v", appErr)
	}
}
