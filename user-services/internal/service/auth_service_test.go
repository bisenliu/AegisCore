package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
)

var authTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestAuthServiceLogin(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)

	tokens, err := svc.Login(context.Background(), dto.LoginRequest{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.TokenType != auth.TokenTypeBearer || tokens.ExpiresIn != 900 {
		t.Fatalf("tokens = %#v", tokens)
	}
	if repo.gotUsername != "alice" {
		t.Fatalf("gotUsername = %q", repo.gotUsername)
	}
	if store.created.SessionID == "" || store.created.UserID != authTestUserID.String() || store.created.TokenVersion != 2 {
		t.Fatalf("created session = %#v", store.created)
	}
}

func TestAuthServiceLoginUsesDefaultTTLs(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthServiceWithConfig(repo, store, config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}})

	tokens, err := svc.Login(context.Background(), dto.LoginRequest{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.ExpiresIn != int64(defaultAccessTokenTTL.Seconds()) {
		t.Fatalf("ExpiresIn = %d, want %d", tokens.ExpiresIn, int64(defaultAccessTokenTTL.Seconds()))
	}
	if store.createdTTL != defaultRefreshTokenTTL {
		t.Fatalf("created TTL = %s, want %s", store.createdTTL, defaultRefreshTokenTTL)
	}
}

func TestAuthServiceLoginUsesExplicitTTLs(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	accessTTL := time.Minute
	refreshTTL := 2 * time.Hour
	svc := newTestAuthServiceWithConfig(repo, store, config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: accessTTL, RefreshTokenTTL: refreshTTL}})

	tokens, err := svc.Login(context.Background(), dto.LoginRequest{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.ExpiresIn != int64(accessTTL.Seconds()) {
		t.Fatalf("ExpiresIn = %d, want %d", tokens.ExpiresIn, int64(accessTTL.Seconds()))
	}
	if store.createdTTL != refreshTTL {
		t.Fatalf("created TTL = %s, want %s", store.createdTTL, refreshTTL)
	}
}

func TestAuthServiceLoginRejectsInvalidCredentials(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	svc := newTestAuthService(&authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusNormal, TokenVersion: 2}}, &sessionStoreStub{version: 2}, true)

	_, err = svc.Login(context.Background(), dto.LoginRequest{Username: "alice", Password: "wrong"})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeUnauthenticated {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceLoginRejectsInactiveStatuses(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	for _, status := range []domain.UserStatus{domain.UserStatusDisabled} {
		svc := newTestAuthService(&authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: status, TokenVersion: 2}}, &sessionStoreStub{version: 2}, true)

		_, err = svc.Login(context.Background(), dto.LoginRequest{Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeUnauthenticated {
			t.Fatalf("status %d err = %#v", status, appErr)
		}
	}
}

func TestAuthServiceLoginIssuesPasswordChangeToken(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusMustChangePassword, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)

	tokens, err := svc.Login(context.Background(), dto.LoginRequest{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken != "" || !tokens.PasswordChangeRequired {
		t.Fatalf("tokens = %#v", tokens)
	}
	claims, err := testJWTService().ParsePasswordChangeToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("ParsePasswordChangeToken: %v", err)
	}
	if claims.UserID != authTestUserID.String() || claims.TokenVersion != 2 || claims.Subject != auth.SubjectPasswordChange {
		t.Fatalf("claims = %#v", claims)
	}
	if store.created.SessionID != "" {
		t.Fatalf("created normal session = %#v", store.created)
	}
}

func TestAuthServiceChangePassword(t *testing.T) {
	passwordHash, err := password.Hash("old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByID: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)
	token, err := testJWTService().SignPasswordChangeToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	result, err := svc.ChangePassword(context.Background(), dto.ChangePasswordRequest{Token: token, NewPassword: "new-secret"})

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if !result.Changed || repo.updatedInput.UserID != authTestUserID || repo.updatedInput.Status != domain.UserStatusNormal || repo.incrementedUserID != authTestUserID || !store.invalidated || !store.deletedAll {
		t.Fatalf("result=%#v repo=%#v store=%#v", result, repo, store)
	}
	matched, err := password.Verify("new-secret", repo.updatedInput.PasswordHash)
	if err != nil || !matched {
		t.Fatalf("updated password hash mismatch: matched=%v err=%v", matched, err)
	}
}

func TestAuthServiceChangePasswordMapsCredentialUpdateNotFound(t *testing.T) {
	repo := &authRepoStub{userByID: &domain.User{ID: 123, UserID: authTestUserID, Status: domain.UserStatusMustChangePassword, TokenVersion: 2}, updateErr: domain.ErrUserNotFound}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)
	token, err := testJWTService().SignPasswordChangeToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	_, err = svc.ChangePassword(context.Background(), dto.ChangePasswordRequest{Token: token, NewPassword: "new-secret"})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeNotFound {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceChangePasswordMapsTokenVersionUserNotFound(t *testing.T) {
	svc := newTestAuthService(&authRepoStub{}, &sessionStoreStub{getVersionErr: domain.ErrUserNotFound}, true)
	token, err := testJWTService().SignPasswordChangeToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	_, err = svc.ChangePassword(context.Background(), dto.ChangePasswordRequest{Token: token, NewPassword: "new-secret"})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeNotFound {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceChangePasswordRejectsAccessToken(t *testing.T) {
	repo := &authRepoStub{userByID: &domain.User{ID: 123, UserID: authTestUserID, Status: domain.UserStatusMustChangePassword, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)
	token, err := testJWTService().SignAccessToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	_, err = svc.ChangePassword(context.Background(), dto.ChangePasswordRequest{Token: token, NewPassword: "new-secret"})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceRefreshRotatesSession(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: repository.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, true)
	refresh, err := testJWTService().SignRefreshToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	tokens, err := svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: refresh})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("tokens = %#v", tokens)
	}
	if !store.deleted || store.deletedSessionID != "s-old" {
		t.Fatalf("deleted = %v, session=%q", store.deleted, store.deletedSessionID)
	}
	if store.created.SessionID == "" || store.created.SessionID == "s-old" {
		t.Fatalf("created rotated session = %#v", store.created)
	}
}

func TestAuthServiceRefreshUsesNormalizedToken(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: repository.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, false)
	refresh, err := testJWTService().SignRefreshToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	tokens, err := svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: refresh})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthServiceRefreshRejectsAccessTokenSubject(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: repository.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, false)
	access, err := testJWTService().SignAccessToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: access})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceRefreshRejectsVersionChange(t *testing.T) {
	store := &sessionStoreStub{version: 3, session: repository.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, true)
	refresh, err := testJWTService().SignRefreshToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: refresh})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceRefreshMapsTokenVersionUserNotFound(t *testing.T) {
	store := &sessionStoreStub{session: repository.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}, getVersionErr: domain.ErrUserNotFound}
	svc := newTestAuthService(&authRepoStub{}, store, true)
	refresh, err := testJWTService().SignRefreshToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: refresh})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeNotFound {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceLogoutAllIncrementsVersionAndDeletesSessions(t *testing.T) {
	repo := &authRepoStub{newVersion: 3}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)
	ctx := auth.WithSessionID(auth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	result, err := svc.LogoutAll(ctx)

	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if !result.LoggedOut || repo.incrementedUserID != authTestUserID || !store.invalidated || !store.deletedAll {
		t.Fatalf("result=%#v repo=%#v store=%#v", result, repo, store)
	}
}

func TestAuthServiceLogoutAllMapsIncrementUserNotFound(t *testing.T) {
	repo := &authRepoStub{incrementErr: domain.ErrUserNotFound}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)
	ctx := auth.WithSessionID(auth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	_, err := svc.LogoutAll(ctx)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeNotFound {
		t.Fatalf("err = %#v", appErr)
	}
	if store.invalidated || store.deletedAll {
		t.Fatalf("sessions mutated after increment failure: %#v", store)
	}
}

func newTestAuthService(repo *authRepoStub, store repository.AuthSessionRepository, rotation bool) AuthService {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: rotation, TokenVersionCacheTTL: time.Minute}}
	return NewAuthService(AuthServiceParams{Credentials: repo, TokenVersions: repo, Sessions: store, JWT: auth.NewJWTService(cfg.Auth), Config: cfg})
}

func newTestAuthServiceWithConfig(repo *authRepoStub, store repository.AuthSessionRepository, authCfg config.AuthConfig) AuthService {
	cfg := &config.Config{Auth: authCfg}
	return NewAuthService(AuthServiceParams{Credentials: repo, TokenVersions: repo, Sessions: store, JWT: auth.NewJWTService(cfg.Auth), Config: cfg})
}

func testJWTService() *auth.JWTService {
	return auth.NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}})
}

type authRepoStub struct {
	userByUsername    *domain.User
	userByID          *domain.User
	gotUsername       string
	newVersion        int64
	incrementErr      error
	updateErr         error
	incrementedUserID uuid.UUID
	updatedInput      repository.UpdateCredentialsInput
}

func (r *authRepoStub) GetByUserID(_ context.Context, userID uuid.UUID) (*domain.User, error) {
	if r.userByID == nil {
		return nil, domain.ErrUserNotFound
	}
	return r.userByID, nil
}
func (r *authRepoStub) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	r.gotUsername = username
	if r.userByUsername == nil {
		return nil, domain.ErrUserNotFound
	}
	return r.userByUsername, nil
}
func (r *authRepoStub) GetTokenVersion(context.Context, uuid.UUID) (int64, error) { return 0, nil }
func (r *authRepoStub) IncrementTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	r.incrementedUserID = userID
	if r.incrementErr != nil {
		return 0, r.incrementErr
	}
	return r.newVersion, nil
}
func (r *authRepoStub) UpdateCredentials(_ context.Context, input repository.UpdateCredentialsInput) (int64, error) {
	r.updatedInput = input
	if r.updateErr != nil {
		return 0, r.updateErr
	}
	return r.newVersion, nil
}

type sessionStoreStub struct {
	version          int64
	session          repository.AuthSession
	created          repository.AuthSession
	createdTTL       time.Duration
	deleted          bool
	deletedSessionID string
	deletedAll       bool
	invalidated      bool
	getVersionErr    error
	invalidateErr    error
	deleteAllErr     error
}

func (s *sessionStoreStub) GetCurrentTokenVersion(context.Context, string) (int64, error) {
	if s.getVersionErr != nil {
		return 0, s.getVersionErr
	}
	return s.version, nil
}
func (s *sessionStoreStub) ValidateTokenVersion(context.Context, string, int64) error { return nil }
func (s *sessionStoreStub) CreateSession(_ context.Context, session repository.AuthSession, ttl time.Duration) error {
	s.created = session
	s.createdTTL = ttl
	return nil
}
func (s *sessionStoreStub) GetSession(context.Context, string) (repository.AuthSession, error) {
	if s.session.SessionID == "" {
		return repository.AuthSession{}, repository.ErrAuthSessionNotFound
	}
	return s.session, nil
}
func (s *sessionStoreStub) DeleteSession(_ context.Context, _ string, sessionID string) error {
	if sessionID == "error" {
		return errors.New("delete failed")
	}
	s.deleted = true
	s.deletedSessionID = sessionID
	return nil
}
func (s *sessionStoreStub) DeleteAllUserSessions(context.Context, string) error {
	if s.deleteAllErr != nil {
		return s.deleteAllErr
	}
	s.deletedAll = true
	return nil
}
func (s *sessionStoreStub) InvalidateUserTokenVersion(context.Context, string) error {
	if s.invalidateErr != nil {
		return s.invalidateErr
	}
	s.invalidated = true
	return nil
}
