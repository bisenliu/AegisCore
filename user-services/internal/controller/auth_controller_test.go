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
	"github.com/aegiscore/user-services/internal/api/auth"
	"github.com/gin-gonic/gin"
)

func TestAuthControllerLoginPassesBoundRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{tokens: &authapi.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthLogin(t, service, `{"username":" alice ","password":" secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotLogin.Username != " alice " || service.gotLogin.Password != " secret " {
		t.Fatalf("gotLogin = %#v", service.gotLogin)
	}
	if !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerLoginLeavesCredentialNormalizationToService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{tokens: &authapi.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthLogin(t, service, `{"username":"alice","password":" "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotLogin.Username != "alice" || service.gotLogin.Password != " " || !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("service=%#v envelope=%#v", service, envelope)
	}
}

func TestAuthControllerChangePasswordPassesBoundRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{changeResponse: &authapi.ChangePasswordResponse{Changed: true}}

	status, envelope := executeAuthChangePassword(t, service, auth.TokenPrefix+"password-token", `{"new_password":" new-secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotChange.Token != auth.TokenPrefix+"password-token" || service.gotChange.NewPassword != " new-secret " {
		t.Fatalf("gotChange = %#v", service.gotChange)
	}
	if !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshPassesBoundRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{tokens: &authapi.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthRefresh(t, service, `{"refresh_token":" Bearer refresh-token "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotRefresh.RefreshToken != " Bearer refresh-token " {
		t.Fatalf("gotRefresh = %#v", service.gotRefresh)
	}
	if !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshLeavesTokenNormalizationToService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthService{tokens: &authapi.TokenResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: auth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthRefresh(t, service, `{"refresh_token":" Bearer "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotRefresh.RefreshToken != " Bearer " || !envelope.Success || envelope.Code != response.CodeOK {
		t.Fatalf("service=%#v envelope=%#v", service, envelope)
	}
}

type stubAuthService struct {
	tokens         *authapi.TokenResponse
	changeResponse *authapi.ChangePasswordResponse
	logoutResponse *authapi.LogoutResponse
	gotLogin       authapi.LoginRequest
	gotRefresh     authapi.RefreshTokenRequest
	gotChange      authapi.ChangePasswordRequest
}

func (s *stubAuthService) Login(_ context.Context, req authapi.LoginRequest) (*authapi.TokenResponse, error) {
	s.gotLogin = req
	return s.tokens, nil
}

func (s *stubAuthService) ChangePassword(_ context.Context, req authapi.ChangePasswordRequest) (*authapi.ChangePasswordResponse, error) {
	s.gotChange = req
	return s.changeResponse, nil
}

func (s *stubAuthService) Refresh(_ context.Context, req authapi.RefreshTokenRequest) (*authapi.TokenResponse, error) {
	s.gotRefresh = req
	return s.tokens, nil
}

func (s *stubAuthService) Logout(context.Context) (*authapi.LogoutResponse, error) {
	return s.logoutResponse, nil
}

func (s *stubAuthService) LogoutAll(context.Context) (*authapi.LogoutResponse, error) {
	return s.logoutResponse, nil
}

func executeAuthLogin(t *testing.T, service *stubAuthService, body string) (int, response.Envelope) {
	t.Helper()
	ctl := newTestAuthController(t, service)
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/login", body)

	ctl.LoginUser(ctx)

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

	ctl.RefreshToken(ctx)

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
