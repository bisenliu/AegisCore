package authhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/validation"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

var errAuthDatabaseDown = errors.New("database down")

func TestAuthControllerLoginNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{tokens: &authcommand.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthLogin(t, service, `{"username":" alice ","password":" secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotLogin.Username != "alice" || service.gotLogin.Password != "secret" {
		t.Fatalf("gotLogin = %#v", service.gotLogin)
	}
	if !envelope.Success || envelope.Code != contracterrors.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok || data["access_token"] != "access" || data["refresh_token"] != "refresh" || data["token_type"] != commonauth.TokenTypeBearer || data["expires_in"] != float64(900) {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestAuthControllerLoginRejectsBlankTrimmedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{tokens: &authcommand.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthLogin(t, service, `{"username":"alice","password":" "}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if service.gotLogin.Username != "" || service.gotLogin.Password != "" || envelope.Success || envelope.Code != contracterrors.CodeUnauthenticated {
		t.Fatalf("service=%#v envelope=%#v", service, envelope)
	}
}

func TestAuthControllerLoginMapsInvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{loginErr: authdomain.ErrInvalidCredentials}

	status, envelope := executeAuthLogin(t, service, `{"username":"alice","password":"secret"}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeUnauthenticated || envelope.Message != messages.InvalidCredentials {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerLoginMapsServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{loginErr: errAuthDatabaseDown}

	status, envelope := executeAuthLogin(t, service, `{"username":"alice","password":"secret"}`)

	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeInternalError || envelope.Message != response.MessageInternalError {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerChangePasswordNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{changeResponse: &authcommand.ChangePasswordResult{Changed: true}}

	status, envelope := executeAuthChangePassword(t, service, commonauth.TokenPrefix+"password-token", `{"new_password":" new-secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotChange.Token != "password-token" || service.gotChange.NewPassword != "new-secret" {
		t.Fatalf("gotChange = %#v", service.gotChange)
	}
	if !envelope.Success || envelope.Code != contracterrors.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok || data["changed"] != true {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestAuthControllerChangePasswordMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{changeErr: userdomain.ErrUserNotFound}

	status, envelope := executeAuthChangePassword(t, service, commonauth.TokenPrefix+"password-token", `{"new_password":"new-secret"}`)

	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", status, http.StatusNotFound)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeNotFound || envelope.Message != messages.UserNotFound {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{tokens: &authcommand.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthRefresh(t, service, `{"refresh_token":" Bearer refresh-token "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if service.gotRefresh.RefreshToken != "refresh-token" {
		t.Fatalf("gotRefresh = %#v", service.gotRefresh)
	}
	if !envelope.Success || envelope.Code != contracterrors.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshRejectsBearerOnlyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{tokens: &authcommand.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}}

	status, envelope := executeAuthRefresh(t, service, `{"refresh_token":" Bearer "}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if service.gotRefresh.RefreshToken != "" || envelope.Success || envelope.Code != contracterrors.CodeTokenInvalid {
		t.Fatalf("service=%#v envelope=%#v", service, envelope)
	}
}

func TestAuthControllerRefreshMapsTokenInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAuthUseCases{refreshErr: authdomain.ErrTokenInvalid}

	status, envelope := executeAuthRefresh(t, service, `{"refresh_token":"refresh-token"}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeTokenInvalid || envelope.Message != messages.MissingSession {
		t.Fatalf("envelope = %#v", envelope)
	}
}

type stubAuthUseCases struct {
	tokens         *authcommand.TokenResult
	loginErr       error
	changeResponse *authcommand.ChangePasswordResult
	changeErr      error
	refreshErr     error
	logoutResponse *authcommand.LogoutResult
	logoutErr      error
	gotLogin       authcommand.LoginCommand
	gotRefresh     authcommand.RefreshTokenCommand
	gotChange      authcommand.ChangePasswordCommand
}

func (s *stubAuthUseCases) Login(_ context.Context, cmd authcommand.LoginCommand) (*authcommand.TokenResult, error) {
	s.gotLogin = cmd
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	return s.tokens, nil
}

func (s *stubAuthUseCases) ChangePassword(_ context.Context, cmd authcommand.ChangePasswordCommand) (*authcommand.ChangePasswordResult, error) {
	s.gotChange = cmd
	if s.changeErr != nil {
		return nil, s.changeErr
	}
	return s.changeResponse, nil
}

func (s *stubAuthUseCases) Refresh(_ context.Context, cmd authcommand.RefreshTokenCommand) (*authcommand.TokenResult, error) {
	s.gotRefresh = cmd
	if s.refreshErr != nil {
		return nil, s.refreshErr
	}
	return s.tokens, nil
}

func (s *stubAuthUseCases) LogoutCurrentSession(context.Context) (*authcommand.LogoutResult, error) {
	if s.logoutErr != nil {
		return nil, s.logoutErr
	}
	return s.logoutResponse, nil
}

func (s *stubAuthUseCases) LogoutAllSessions(context.Context) (*authcommand.LogoutResult, error) {
	if s.logoutErr != nil {
		return nil, s.logoutErr
	}
	return s.logoutResponse, nil
}

func executeAuthLogin(t *testing.T, service *stubAuthUseCases, body string) (int, response.Envelope) {
	t.Helper()
	ctl := newTestAuthController(t, service)
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/login", body)

	ctl.LoginUser(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func executeAuthChangePassword(t *testing.T, service *stubAuthUseCases, token string, body string) (int, response.Envelope) {
	t.Helper()
	ctl := newTestAuthController(t, service)
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/change-password", body)
	ctx.Request.Header.Set(commonauth.AuthorizationHeader, token)

	ctl.ChangePassword(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func executeAuthRefresh(t *testing.T, service *stubAuthUseCases, body string) (int, response.Envelope) {
	t.Helper()
	ctl := newTestAuthController(t, service)
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/refresh", body)

	ctl.RefreshToken(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func newTestAuthController(t *testing.T, service *stubAuthUseCases) *AuthController {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	return NewAuthController(AuthControllerParams{
		Login:          service,
		Refresh:        service,
		ChangePassword: service,
		LogoutCurrent:  service,
		LogoutAll:      service,
		Validator:      validator,
	})
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
