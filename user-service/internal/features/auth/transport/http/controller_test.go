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
	"go.uber.org/mock/gomock"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/messages"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var errAuthDatabaseDown = errors.New("database down")

func TestAuthControllerLoginNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, cmd authcommand.LoginCommand) (*authtokens.TokenResult, error) {
		if cmd.Username != "alice" || cmd.Password != "secret" {
			t.Fatalf("cmd = %#v", cmd)
		}
		clientContext, ok := authctx.ClientContextFromContext(ctx)
		if !ok || clientContext.ClientIP != "203.0.113.20" || clientContext.UserAgent != "auth-controller-test" {
			t.Fatalf("clientContext = %#v, %v", clientContext, ok)
		}
		return &authtokens.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}, nil
	})

	status, envelope := executeAuthLogin(t, ctl, `{"username":" alice ","password":" secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
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
	ctl, _ := newTestAuthController(t)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":" "}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeUnauthenticated {
		t.Fatalf("envelope=%#v", envelope)
	}
}

func TestAuthControllerLoginMapsInvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), authcommand.LoginCommand{Username: "alice", Password: "secret"}).Return(nil, authdomain.ErrInvalidCredentials)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":"secret"}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeUnauthenticated || envelope.Message != messages.InvalidCredentials {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerLoginMapsPasswordKDFBusy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), authcommand.LoginCommand{Username: "alice", Password: "secret"}).Return(nil, password.ErrPasswordKDFBusy)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":"secret"}`)

	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", status, http.StatusServiceUnavailable)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeServiceUnavailable || envelope.Message != messages.AuthServiceBusy {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerLoginMapsServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), authcommand.LoginCommand{Username: "alice", Password: "secret"}).Return(nil, errAuthDatabaseDown)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":"secret"}`)

	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeInternalError || envelope.Message != response.MessageInternalError {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerChangePasswordNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.changePassword.EXPECT().ChangePassword(gomock.Any(), authcommand.ChangePasswordCommand{Token: "password-token", NewPassword: "new-secret"}).Return(&authcommand.ChangePasswordResult{Changed: true}, nil)

	status, envelope := executeAuthChangePassword(t, ctl, commonauth.TokenPrefix+"password-token", `{"new_password":" new-secret "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
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
	ctl, mocks := newTestAuthController(t)
	mocks.changePassword.EXPECT().ChangePassword(gomock.Any(), authcommand.ChangePasswordCommand{Token: "password-token", NewPassword: "new-secret"}).Return(nil, identity.ErrUserNotFound)

	status, envelope := executeAuthChangePassword(t, ctl, commonauth.TokenPrefix+"password-token", `{"new_password":"new-secret"}`)

	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", status, http.StatusNotFound)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeNotFound || envelope.Message != messages.UserNotFound {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.refresh.EXPECT().Refresh(gomock.Any(), authcommand.RefreshTokenCommand{RefreshToken: "refresh-token"}).Return(&authtokens.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}, nil)

	status, envelope := executeAuthRefresh(t, ctl, `{"refresh_token":" Bearer refresh-token "}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if !envelope.Success || envelope.Code != contracterrors.CodeOK {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAuthControllerRefreshRejectsBearerOnlyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, _ := newTestAuthController(t)

	status, envelope := executeAuthRefresh(t, ctl, `{"refresh_token":" Bearer "}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeTokenInvalid {
		t.Fatalf("envelope=%#v", envelope)
	}
}

func TestAuthControllerRefreshMapsTokenInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.refresh.EXPECT().Refresh(gomock.Any(), authcommand.RefreshTokenCommand{RefreshToken: "refresh-token"}).Return(nil, authdomain.ErrTokenInvalid)

	status, envelope := executeAuthRefresh(t, ctl, `{"refresh_token":"refresh-token"}`)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeTokenInvalid || envelope.Message != messages.MissingSession {
		t.Fatalf("envelope = %#v", envelope)
	}
}

type authControllerMocks struct {
	login          *MockLoginUseCase
	refresh        *MockRefreshTokenUseCase
	changePassword *MockChangePasswordUseCase
	logoutCurrent  *MockLogoutCurrentSessionUseCase
	logoutAll      *MockLogoutAllSessionsUseCase
}

func executeAuthLogin(t *testing.T, ctl *AuthController, body string) (int, response.Envelope) {
	t.Helper()
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/login", body)

	ctl.LoginUser(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func executeAuthChangePassword(t *testing.T, ctl *AuthController, token string, body string) (int, response.Envelope) {
	t.Helper()
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/change-password", body)
	ctx.Request.Header.Set(commonauth.AuthorizationHeader, token)

	ctl.ChangePassword(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func executeAuthRefresh(t *testing.T, ctl *AuthController, body string) (int, response.Envelope) {
	t.Helper()
	recorder, ctx := newAuthJSONContext(http.MethodPost, "/api/v1/auth/refresh", body)

	ctl.RefreshToken(ctx)

	return decodeAuthEnvelope(t, recorder)
}

func newTestAuthController(t *testing.T) (*AuthController, authControllerMocks) {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	ctrl := gomock.NewController(t)
	mocks := authControllerMocks{
		login:          NewMockLoginUseCase(ctrl),
		refresh:        NewMockRefreshTokenUseCase(ctrl),
		changePassword: NewMockChangePasswordUseCase(ctrl),
		logoutCurrent:  NewMockLogoutCurrentSessionUseCase(ctrl),
		logoutAll:      NewMockLogoutAllSessionsUseCase(ctrl),
	}
	return NewAuthController(AuthControllerParams{
		Login:          mocks.login,
		Refresh:        mocks.refresh,
		ChangePassword: mocks.changePassword,
		LogoutCurrent:  mocks.logoutCurrent,
		LogoutAll:      mocks.logoutAll,
		Validator:      validator,
	}), mocks
}

func newAuthJSONContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "auth-controller-test")
	request.ContentLength = int64(len(body))
	request.RemoteAddr = "203.0.113.20:1234"
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
