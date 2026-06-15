package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/common/testing/fixtures"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

type userResponse struct {
	UserID    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	Username  string `json:"username"`
	Status    int64  `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type tokenResponse struct {
	AccessToken            string `json:"access_token"`
	RefreshToken           string `json:"refresh_token"`
	TokenType              string `json:"token_type"`
	ExpiresIn              int64  `json:"expires_in"`
	PasswordChangeRequired bool   `json:"password_change_required"`
}

type changePasswordResponse struct {
	Changed bool `json:"changed"`
}

type logoutResponse struct {
	LoggedOut bool `json:"logged_out"`
}

type seededUser struct {
	Username string
	Password string
}

func TestHTTPAuthUserFlow(t *testing.T) {
	harness := newHTTPFlowHarness(t)
	faker := fixtures.NewFaker(t)

	bootstrapUser := seedUser(t, harness, seededUserInput{
		Nickname: faker.Name("Bootstrap Admin"),
		Username: faker.Username("bootstrap-admin"),
		Password: "bootstrap-secret",
		Status:   identity.UserStatusNormal,
	})
	bootstrapTokens := login(t, harness, bootstrapUser.Username, bootstrapUser.Password)

	mustChangePassword := "initial-secret"
	targetUser := createUser(t, harness, bootstrapTokens.AccessToken, map[string]any{
		"nickname": faker.Name("Password Change"),
		"username": faker.Username("password-change"),
		"password": mustChangePassword,
		"status":   int64(identity.UserStatusMustChangePassword),
	})
	getUser(t, harness, bootstrapTokens.AccessToken, targetUser.UserID, targetUser.Username, int64(identity.UserStatusMustChangePassword))

	passwordChangeTokens := login(t, harness, targetUser.Username, mustChangePassword)
	if !passwordChangeTokens.PasswordChangeRequired || passwordChangeTokens.RefreshToken != "" {
		t.Fatalf("password-change login returned password_change_required=%v refresh_present=%v, want true false", passwordChangeTokens.PasswordChangeRequired, passwordChangeTokens.RefreshToken != "")
	}

	newPassword := "changed-secret"
	changePassword(t, harness, passwordChangeTokens.AccessToken, newPassword)
	expectLoginFailure(t, harness, targetUser.Username, mustChangePassword)

	targetTokens := login(t, harness, targetUser.Username, newPassword)
	if targetTokens.PasswordChangeRequired || targetTokens.RefreshToken == "" {
		t.Fatalf("normal login returned password_change_required=%v refresh_present=%v, want false true", targetTokens.PasswordChangeRequired, targetTokens.RefreshToken != "")
	}
	getUser(t, harness, targetTokens.AccessToken, targetUser.UserID, targetUser.Username, int64(identity.UserStatusNormal))

	expectMissingAuthorization(t, harness, targetUser.UserID)
	logoutCurrent(t, harness, targetTokens.AccessToken)
	expectRefreshFailure(t, harness, targetTokens.RefreshToken)
}

type seededUserInput struct {
	Nickname string
	Username string
	Password string
	Status   identity.UserStatus
}

func seedUser(t *testing.T, harness *httpFlowHarness, input seededUserInput) seededUser {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	passwordHash, err := password.HashContext(ctx, input.Password)
	if err != nil {
		t.Fatalf("hash bootstrap password: %v", err)
	}
	userID := uuid.New()
	now := time.Now().UnixMilli()
	db := openPostgres(t, harness.postgresDSN)
	_, err = db.ExecContext(ctx, `
INSERT INTO users (user_id, nickname, username, password_hash, token_version, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, 1, $5, $6, $7)
`, userID, input.Nickname, input.Username, passwordHash, int64(input.Status), now, now)
	if err != nil {
		t.Fatalf("seed bootstrap user: %v", err)
	}
	return seededUser{Username: input.Username, Password: input.Password}
}

func createUser(t *testing.T, harness *httpFlowHarness, token string, body map[string]any) userResponse {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/users", body, token)
	envelope := expectEnvelope(t, recorder, http.StatusCreated, true, contracterrors.CodeOK)
	created := decodeData[userResponse](t, envelope)
	if created.UserID == "" || created.Username != body["username"] {
		t.Fatalf("created user = user_id %q username %q, want username %q", created.UserID, created.Username, body["username"])
	}
	return created
}

func login(t *testing.T, harness *httpFlowHarness, username string, plainPassword string) tokenResponse {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": username,
		"password": plainPassword,
	}, "")
	envelope := expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	tokens := decodeData[tokenResponse](t, envelope)
	if tokens.AccessToken == "" || tokens.TokenType != commonauth.TokenTypeBearer || tokens.ExpiresIn <= 0 {
		t.Fatalf("login token metadata invalid: access_present=%v token_type=%q expires_in=%d", tokens.AccessToken != "", tokens.TokenType, tokens.ExpiresIn)
	}
	return tokens
}

func getUser(t *testing.T, harness *httpFlowHarness, token string, userID string, username string, status int64) userResponse {
	t.Helper()
	recorder := harness.request(t, http.MethodGet, "/api/v1/users/"+userID, nil, token)
	envelope := expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	user := decodeData[userResponse](t, envelope)
	if user.UserID != userID || user.Username != username || user.Status != status {
		t.Fatalf("user response = user_id %q username %q status %d, want user_id %q username %q status %d", user.UserID, user.Username, user.Status, userID, username, status)
	}
	return user
}

func changePassword(t *testing.T, harness *httpFlowHarness, passwordChangeToken string, newPassword string) {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/change-password", map[string]any{
		"new_password": newPassword,
	}, passwordChangeToken)
	envelope := expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	result := decodeData[changePasswordResponse](t, envelope)
	if !result.Changed {
		t.Fatal("change password response changed=false, want true")
	}
}

func logoutCurrent(t *testing.T, harness *httpFlowHarness, token string) {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/logout", nil, token)
	envelope := expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	result := decodeData[logoutResponse](t, envelope)
	if !result.LoggedOut {
		t.Fatal("logout response logged_out=false, want true")
	}
}

func expectLoginFailure(t *testing.T, harness *httpFlowHarness, username string, plainPassword string) {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": username,
		"password": plainPassword,
	}, "")
	expectEnvelope(t, recorder, http.StatusUnauthorized, false, contracterrors.CodeUnauthenticated)
}

func expectMissingAuthorization(t *testing.T, harness *httpFlowHarness, userID string) {
	t.Helper()
	recorder := harness.request(t, http.MethodGet, "/api/v1/users/"+userID, nil, "")
	expectEnvelope(t, recorder, http.StatusUnauthorized, false, contracterrors.CodeUnauthenticated)
}

func expectRefreshFailure(t *testing.T, harness *httpFlowHarness, refreshToken string) {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	}, "")
	expectEnvelope(t, recorder, http.StatusUnauthorized, false, contracterrors.CodeTokenInvalid)
}
