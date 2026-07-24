package e2e

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/common/testing/fixtures"
	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	"github.com/aegiscore/user-service/internal/shared/identity"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
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
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type changePasswordResponse struct {
	Changed bool `json:"changed"`
}

type logoutResponse struct {
	LoggedOut bool `json:"logged_out"`
}

func TestHTTPAuthUserFlow(t *testing.T) {
	harness := newHTTPFlowHarness(t)
	faker := fixtures.NewFaker(t)

	bootstrapPasswordChangeTokens := loginPasswordChangeRequired(t, harness, "initial-admin", "bootstrap-secret")
	bootstrapPassword := "changed-bootstrap-secret"
	changePassword(t, harness, bootstrapPasswordChangeTokens.AccessToken, bootstrapPassword)
	bootstrapTokens := login(t, harness, "initial-admin", bootstrapPassword)
	delegatedAdminPassword := "delegated-admin-secret"
	delegatedAdmin := createUser(t, harness, bootstrapTokens.AccessToken, map[string]any{
		"nickname": faker.Name("Delegated Admin"),
		"username": faker.Username("delegated-admin"),
		"password": delegatedAdminPassword,
		"status":   int64(identity.UserStatusNormal),
	})
	addUserRole(t, harness, bootstrapTokens.AccessToken, delegatedAdmin.UserID, rbacbaseline.SuperAdminRoleID)
	delegatedTokens := login(t, harness, delegatedAdmin.Username, delegatedAdminPassword)

	mustChangePassword := "initial-secret"
	targetUser := createUser(t, harness, delegatedTokens.AccessToken, map[string]any{
		"nickname": faker.Name("Password Change"),
		"username": faker.Username("password-change"),
		"password": mustChangePassword,
		"status":   int64(identity.UserStatusMustChangePassword),
	})
	bindSuperAdminRoleByUserID(t, harness.postgresDSN, targetUser.UserID)
	getUser(t, harness, bootstrapTokens.AccessToken, targetUser.UserID, targetUser.Username, int64(identity.UserStatusMustChangePassword))

	passwordChangeTokens := loginPasswordChangeRequired(t, harness, targetUser.Username, mustChangePassword)

	newPassword := "changed-secret"
	changePassword(t, harness, passwordChangeTokens.AccessToken, newPassword)
	expectLoginFailure(t, harness, targetUser.Username, mustChangePassword)

	targetTokens := login(t, harness, targetUser.Username, newPassword)
	require.NotEmpty(t, targetTokens.RefreshToken, "normal login returned empty refresh token")
	getUser(t, harness, targetTokens.AccessToken, targetUser.UserID, targetUser.Username, int64(identity.UserStatusNormal))

	expectMissingAuthorization(t, harness, targetUser.UserID)
	logoutCurrent(t, harness, targetTokens.AccessToken)
	expectRefreshFailure(t, harness, targetTokens.RefreshToken)
}

func bootstrapSuperAdmin(t *testing.T, dsn string, plainPassword string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	passwordService, err := password.NewService()
	require.NoError(t, err, "create password service")
	db := openPostgres(t, dsn)
	store := rolepostgres.NewBootstrapStore(db)
	service := rolebootstrap.NewService(store, passwordService)
	_, err = service.BootstrapSuperAdmin(ctx, rolebootstrap.Command{Username: "initial-admin", Nickname: "Initial Administrator", Password: plainPassword})
	require.NoError(t, err, "bootstrap super admin")
}
func seedRBACBaseline(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db := openPostgres(t, dsn)
	now := time.Now().UnixMilli()

	for _, role := range rbacbaseline.DefaultRoles() {
		_, err := db.ExecContext(ctx, `
INSERT INTO roles (role_id, name, description, active, is_system, created_at, updated_at)
VALUES ($1, $2, $3, true, $4, $5, $5)
ON CONFLICT (role_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, active = true, is_system = EXCLUDED.is_system, updated_at = EXCLUDED.updated_at
`, role.RoleID, role.Name, role.Description, role.System, now)
		require.NoError(t, err, "seed RBAC role %s", role.RoleID)
	}
	for _, permission := range rbacbaseline.DefaultPermissions() {
		_, err := db.ExecContext(ctx, `
INSERT INTO permissions (permission_id, name, description, module, http_method, path_template, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
ON CONFLICT (permission_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, module = EXCLUDED.module, http_method = EXCLUDED.http_method, path_template = EXCLUDED.path_template, updated_at = EXCLUDED.updated_at
`, permission.PermissionID, permission.Name, permission.Description, permission.Module, permission.Method, permission.PathTemplate, now)
		require.NoError(t, err, "seed RBAC permission %s", permission.PermissionID)
	}
	for _, binding := range rbacbaseline.DefaultRolePermissions() {
		_, err := db.ExecContext(ctx, `
INSERT INTO role_permissions (role_id, permission_id, created_at)
SELECT r.id, p.id, $3
FROM roles r, permissions p
WHERE r.role_id = $1 AND p.permission_id = $2
ON CONFLICT (role_id, permission_id) DO NOTHING
`, binding.RoleID, binding.PermissionID, now)
		require.NoError(t, err, "seed RBAC role permission %s:%s", binding.RoleID, binding.PermissionID)
	}
}

func bindSuperAdminRole(t *testing.T, db *sql.DB, userInternalID int64, now int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `
INSERT INTO user_roles (user_id, role_id, created_at)
SELECT $1, r.id, $2
FROM roles r
WHERE r.role_id = $3
ON CONFLICT (user_id, role_id) DO NOTHING
`, userInternalID, now, rbacbaseline.SuperAdminRoleID)
	require.NoError(t, err, "bind bootstrap user super admin role")
}

func bindSuperAdminRoleByUserID(t *testing.T, dsn string, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db := openPostgres(t, dsn)
	var userInternalID int64
	err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE user_id = $1`, userID).Scan(&userInternalID)
	require.NoError(t, err, "load user internal id")
	bindSuperAdminRole(t, db, userInternalID, time.Now().UnixMilli())
}

func createUser(t *testing.T, harness *httpFlowHarness, token string, body map[string]any) userResponse {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/users", body, token)
	envelope := expectEnvelope(t, recorder, http.StatusCreated, true, contracterrors.CodeOK)
	created := decodeData[userResponse](t, envelope)
	require.NotEmpty(t, created.UserID, "created user_id")
	require.Equal(t, body["username"], created.Username, "created username")
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
	require.NotEmpty(t, tokens.AccessToken, "login access token")
	assert.Equal(t, commonauth.TokenTypeBearer, tokens.TokenType, "login token type")
	assert.Greater(t, tokens.ExpiresIn, int64(0), "login expires_in")
	return tokens
}

func loginPasswordChangeRequired(t *testing.T, harness *httpFlowHarness, username string, plainPassword string) tokenResponse {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": username,
		"password": plainPassword,
	}, "")
	envelope := expectEnvelope(t, recorder, http.StatusOK, false, contracterrors.CodePasswordChangeRequired)
	tokens := decodeData[tokenResponse](t, envelope)
	require.NotEmpty(t, tokens.AccessToken, "password-change access token")
	assert.Equal(t, commonauth.TokenTypeBearer, tokens.TokenType, "password-change token type")
	assert.Greater(t, tokens.ExpiresIn, int64(0), "password-change expires_in")
	assert.Empty(t, tokens.RefreshToken, "password-change refresh token")
	return tokens
}

func getUser(t *testing.T, harness *httpFlowHarness, token string, userID string, username string, status int64) userResponse {
	t.Helper()
	recorder := harness.request(t, http.MethodGet, "/api/v1/users/"+userID, nil, token)
	envelope := expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	user := decodeData[userResponse](t, envelope)
	assert.Equal(t, userID, user.UserID, "user response user_id")
	assert.Equal(t, username, user.Username, "user response username")
	assert.Equal(t, status, user.Status, "user response status")
	return user
}

func changePassword(t *testing.T, harness *httpFlowHarness, passwordChangeToken string, newPassword string) {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/change-password", map[string]any{
		"new_password": newPassword,
	}, passwordChangeToken)
	envelope := expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	result := decodeData[changePasswordResponse](t, envelope)
	require.Equal(t, true, result.Changed, "change password response changed")
}

func addUserRole(t *testing.T, harness *httpFlowHarness, token string, userID string, roleID string) {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/users/"+userID+"/roles", map[string]any{
		"role_id": roleID,
	}, token)
	expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
}

func logoutCurrent(t *testing.T, harness *httpFlowHarness, token string) {
	t.Helper()
	recorder := harness.request(t, http.MethodPost, "/api/v1/auth/logout", nil, token)
	envelope := expectEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	result := decodeData[logoutResponse](t, envelope)
	require.Equal(t, true, result.LoggedOut, "logout response logged_out")
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
