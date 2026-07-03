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
	"github.com/stretchr/testify/require"
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
		require.False(t, cmd.Username != "alice" || cmd.Password != "secret",
			"cmd = %#v", cmd)

		clientContext, ok := authctx.ClientContextFromContext(ctx)
		require.False(t, !ok || clientContext.ClientIP != "203.0.113.20" || clientContext.UserAgent != "auth-controller-test",
			"clientContext = %#v, %v", clientContext, ok)

		return &authtokens.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}, nil
	})

	status, envelope := executeAuthLogin(t, ctl, `{"username":" alice ","password":" secret "}`)
	require.Equal(t, http.StatusOK, status,
		"status = %d, want %d", status, http.StatusOK)
	require.False(t, !envelope.Success || envelope.Code != contracterrors.CodeOK,
		"envelope = %#v", envelope)

	data, ok := envelope.Data.(map[string]any)
	require.False(t, !ok || data["access_token"] != "access" || data["refresh_token"] != "refresh" || data["token_type"] != commonauth.TokenTypeBearer || data["expires_in"] != float64(900),
		"data = %#v", envelope.Data)

}

func TestAuthControllerLoginMapsPasswordChangeRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), authcommand.LoginCommand{Username: "alice", Password: "secret"}).Return(&authtokens.TokenResult{AccessToken: "password-change", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900, PasswordChangeRequired: true}, nil)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":"secret"}`)
	require.Equal(t, http.StatusOK, status,
		"status = %d, want %d", status, http.StatusOK)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodePasswordChangeRequired || envelope.Message != messages.PasswordChangeRequired,
		"envelope = %#v", envelope)

	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok,
		"data = %#v", envelope.Data)
	require.False(t, data["access_token"] != "password-change" || data["token_type"] != commonauth.TokenTypeBearer || data["expires_in"] != float64(900),
		"data = %#v", envelope.Data)
	{

		_, ok := data["refresh_token"]
		require.False(t, ok,
			"refresh_token = %#v, want omitted", data["refresh_token"])
	}
	{

		_, ok := data["password_change_required"]
		require.False(t, ok,
			"password_change_required = %#v, want omitted", data["password_change_required"])
	}

}

func TestAuthControllerLoginRejectsBlankTrimmedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, _ := newTestAuthController(t)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":" "}`)
	require.Equal(t, http.StatusUnauthorized, status,
		"status = %d, want %d", status, http.StatusUnauthorized)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodeUnauthenticated,
		"envelope=%#v", envelope)

}

func TestAuthControllerLoginMapsInvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), authcommand.LoginCommand{Username: "alice", Password: "secret"}).Return(nil, authdomain.ErrInvalidCredentials)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":"secret"}`)
	require.Equal(t, http.StatusUnauthorized, status,
		"status = %d, want %d", status, http.StatusUnauthorized)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodeUnauthenticated || envelope.Message != messages.InvalidCredentials,
		"envelope = %#v", envelope)

}

func TestAuthControllerLoginMapsPasswordKDFBusy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), authcommand.LoginCommand{Username: "alice", Password: "secret"}).Return(nil, password.ErrPasswordKDFBusy)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":"secret"}`)
	require.Equal(t, http.StatusServiceUnavailable, status,
		"status = %d, want %d", status, http.StatusServiceUnavailable)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodeServiceUnavailable || envelope.Message != messages.AuthServiceBusy,
		"envelope = %#v", envelope)

}

func TestAuthControllerLoginMapsServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.login.EXPECT().Login(gomock.Any(), authcommand.LoginCommand{Username: "alice", Password: "secret"}).Return(nil, errAuthDatabaseDown)

	status, envelope := executeAuthLogin(t, ctl, `{"username":"alice","password":"secret"}`)
	require.Equal(t, http.StatusInternalServerError, status,
		"status = %d, want %d", status, http.StatusInternalServerError)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodeInternalError || envelope.Message != response.MessageInternalError,
		"envelope = %#v", envelope)

}

func TestAuthControllerChangePasswordNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.changePassword.EXPECT().ChangePassword(gomock.Any(), authcommand.ChangePasswordCommand{Token: "password-token", NewPassword: "new-secret"}).Return(&authcommand.ChangePasswordResult{Changed: true}, nil)

	status, envelope := executeAuthChangePassword(t, ctl, commonauth.TokenPrefix+"password-token", `{"new_password":" new-secret "}`)
	require.Equal(t, http.StatusOK, status,
		"status = %d, want %d", status, http.StatusOK)
	require.False(t, !envelope.Success || envelope.Code != contracterrors.CodeOK,
		"envelope = %#v", envelope)

	data, ok := envelope.Data.(map[string]any)
	require.False(t, !ok || data["changed"] != true,
		"data = %#v", envelope.Data)

}

func TestAuthControllerChangePasswordMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.changePassword.EXPECT().ChangePassword(gomock.Any(), authcommand.ChangePasswordCommand{Token: "password-token", NewPassword: "new-secret"}).Return(nil, identity.ErrUserNotFound)

	status, envelope := executeAuthChangePassword(t, ctl, commonauth.TokenPrefix+"password-token", `{"new_password":"new-secret"}`)
	require.Equal(t, http.StatusNotFound, status,
		"status = %d, want %d", status, http.StatusNotFound)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodeNotFound || envelope.Message != messages.UserNotFound,
		"envelope = %#v", envelope)

}

func TestAuthControllerRefreshNormalizesToCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.refresh.EXPECT().Refresh(gomock.Any(), authcommand.RefreshTokenCommand{RefreshToken: "refresh-token"}).Return(&authtokens.TokenResult{AccessToken: "access", RefreshToken: "refresh", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900}, nil)

	status, envelope := executeAuthRefresh(t, ctl, `{"refresh_token":" Bearer refresh-token "}`)
	require.Equal(t, http.StatusOK, status,
		"status = %d, want %d", status, http.StatusOK)
	require.False(t, !envelope.Success || envelope.Code != contracterrors.CodeOK,
		"envelope = %#v", envelope)

}

func TestAuthControllerRefreshRejectsBearerOnlyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, _ := newTestAuthController(t)

	status, envelope := executeAuthRefresh(t, ctl, `{"refresh_token":" Bearer "}`)
	require.Equal(t, http.StatusUnauthorized, status,
		"status = %d, want %d", status, http.StatusUnauthorized)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodeTokenInvalid,
		"envelope=%#v", envelope)

}

func TestAuthControllerRefreshMapsTokenInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctl, mocks := newTestAuthController(t)
	mocks.refresh.EXPECT().Refresh(gomock.Any(), authcommand.RefreshTokenCommand{RefreshToken: "refresh-token"}).Return(nil, authdomain.ErrTokenInvalid)

	status, envelope := executeAuthRefresh(t, ctl, `{"refresh_token":"refresh-token"}`)
	require.Equal(t, http.StatusUnauthorized, status,
		"status = %d, want %d", status, http.StatusUnauthorized)
	require.False(t, envelope.Success || envelope.Code != contracterrors.CodeTokenInvalid || envelope.Message != messages.MissingSession,
		"envelope = %#v", envelope)

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
	require.NoError(t, err,
		"NewDefault: %v", err)

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
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &envelope)
		require.NoError(t, err,
			"unmarshal response: %v", err)
	}

	return recorder.Code, envelope
}
