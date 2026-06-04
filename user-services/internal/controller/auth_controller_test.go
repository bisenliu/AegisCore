package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/errmsg"
	"github.com/gin-gonic/gin"
)

func TestAuthControllerLoginNormalizesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{tokens: &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthLogin(t, service, `{"username":" alice ","password":" secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotLogin.Username != "alice" || service.gotLogin.Password != "secret" {
		t.Fatalf("gotLogin = %#v", service.gotLogin)
	}
	if !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerLoginRejectsBlankTrimmedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{}

	status, envelope := executeAuthLogin(t, service, `{"username":"alice","password":" "}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if service.gotLogin.Username != "" || envelope.Success || envelope.Code != response.CodeUnauthenticated || envelope.Message != errmsg.MsgInvalidCredentials {
		t.Fatalf("service=%#v envelope=%#v", service, envelope)
	}
}

func TestAuthControllerChangePasswordNormalizesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{changeResponse: &dto.ChangePasswordResponse{Changed: true}}

	status, envelope := executeAuthChangePassword(t, service, auth.TokenPrefix+"password-token", `{"new_password":" new-secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotChange.Token != "password-token" || service.gotChange.NewPassword != "new-secret" {
		t.Fatalf("gotChange = %#v", service.gotChange)
	}
	if !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshNormalizesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{tokens: &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthRefresh(t, service, `{"refresh_token":" Bearer refresh-token "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotRefresh.RefreshToken != "refresh-token" {
		t.Fatalf("gotRefresh = %#v", service.gotRefresh)
	}
	if !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshRejectsBlankTrimmedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{}

	status, envelope := executeAuthRefresh(t, service, `{"refresh_token":" Bearer "}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if service.gotRefresh.RefreshToken != "" || envelope.Success || envelope.Code != response.CodeTokenInvalid || envelope.Message != errmsg.MsgMissingSession {
		t.Fatalf("service=%#v envelope=%#v", service, envelope)
	}
}

type stubAuthService struct {
	tokens         *dto.TokenResponse
	changeResponse *dto.ChangePasswordResponse
	logoutResponse *dto.LogoutResponse
	gotLogin       dto.LoginRequest
	gotRefresh     dto.RefreshTokenRequest
	gotChange      dto.ChangePasswordRequest
}

func (s *stubAuthService) Login(_ context.Context, req dto.LoginRequest) (*dto.TokenResponse, error) {
	s.gotLogin = req
	return s.tokens, nil
}

func (s *stubAuthService) ChangePassword(_ context.Context, req dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	s.gotChange = req
	return s.changeResponse, nil
}

func (s *stubAuthService) Refresh(_ context.Context, req dto.RefreshTokenRequest) (*dto.TokenResponse, error) {
	s.gotRefresh = req
	return s.tokens, nil
}

func (s *stubAuthService) Logout(context.Context) (*dto.LogoutResponse, error) {
	return s.logoutResponse, nil
}

func (s *stubAuthService) LogoutAll(context.Context) (*dto.LogoutResponse, error) {
	return s.logoutResponse, nil
}

func executeAuthLogin(t *testing.T, service *stubAuthService, body string) (int, response.Envelope) {
	t.Helper()
	ctl := newTestAuthController(t, service)
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/login", body)

	ctl.Login(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func executeAuthChangePassword(t *testing.T, service *stubAuthService, token string, body string) (int, response.Envelope) {
	t.Helper()
	ctl := newTestAuthController(t, service)
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/change-password", body)
	ctx.Request.Header.Set(auth.AuthorizationHeader, token)

	ctl.ChangePassword(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func executeAuthRefresh(t *testing.T, service *stubAuthService, body string) (int, response.Envelope) {
	t.Helper()
	ctl := newTestAuthController(t, service)
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/refresh", body)

	ctl.Refresh(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func newTestAuthController(t *testing.T, service *stubAuthService) *AuthController {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	return NewAuthController(service, validator)
}

func newAuthJSONContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = int64(len(body))
	ctx.Request = request
	return recorder, ctx
}

func decodeAuthEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) (int, response.Envelope) {
	t.Helper()
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}
