package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/contextutil"
	commonjwt "github.com/aegiscore/common/jwt"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/aegiscore/user-services/internal/security"
)

func TestAuthServiceLogin(t *testing.T) {
	passwordHash, err := security.HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	repo := &authRepoStub{userByEmail: &ent.User{ID: 123, Email: "alice@example.com", Password: passwordHash, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)

	tokens, err := svc.Login(context.Background(), dto.LoginRequest{Email: " ALICE@example.com ", Password: " secret "})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.TokenType != "Bearer" || tokens.ExpiresIn != 900 {
		t.Fatalf("tokens = %#v", tokens)
	}
	if repo.gotEmail != "alice@example.com" {
		t.Fatalf("gotEmail = %q", repo.gotEmail)
	}
	if store.created.SessionID == "" || store.created.UserID != 123 || store.created.TokenVersion != 2 {
		t.Fatalf("created session = %#v", store.created)
	}
}

func TestAuthServiceLoginRejectsInvalidCredentials(t *testing.T) {
	passwordHash, err := security.HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	svc := newTestAuthService(&authRepoStub{userByEmail: &ent.User{ID: 123, Email: "alice@example.com", Password: passwordHash, TokenVersion: 2}}, &sessionStoreStub{version: 2}, true)

	_, err = svc.Login(context.Background(), dto.LoginRequest{Email: "alice@example.com", Password: "wrong"})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeUnauthenticated {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceRefreshRotatesSession(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: Session{UserID: 123, SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, true)
	refresh, err := svc.(*authService).jwt.SignRefreshToken(commonjwt.SignInput{UserID: "123", TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
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

func TestAuthServiceRefreshAcceptsBearerPrefix(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: Session{UserID: 123, SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, false)
	refresh, err := svc.(*authService).jwt.SignRefreshToken(commonjwt.SignInput{UserID: "123", TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	tokens, err := svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: " Bearer " + refresh + " "})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthServiceRefreshRejectsEmptyBearerPrefix(t *testing.T) {
	svc := newTestAuthService(&authRepoStub{}, &sessionStoreStub{version: 2}, false)

	_, err := svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: " Bearer "})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceRefreshRejectsAccessTokenSubject(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: Session{UserID: 123, SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, false)
	access, err := svc.(*authService).jwt.SignAccessToken(commonjwt.SignInput{UserID: "123", TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
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
	store := &sessionStoreStub{version: 3, session: Session{UserID: 123, SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthService(&authRepoStub{}, store, true)
	refresh, err := svc.(*authService).jwt.SignRefreshToken(commonjwt.SignInput{UserID: "123", TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), dto.RefreshTokenRequest{RefreshToken: refresh})

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthServiceLogoutAllIncrementsVersionAndDeletesSessions(t *testing.T) {
	repo := &authRepoStub{newVersion: 3}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthService(repo, store, true)
	ctx := contextutil.WithSessionID(contextutil.WithUserID(context.Background(), "123"), "s-123")

	result, err := svc.LogoutAll(ctx)

	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if !result.LoggedOut || repo.incrementedUserID != 123 || !store.invalidated || !store.deletedAll {
		t.Fatalf("result=%#v repo=%#v store=%#v", result, repo, store)
	}
}

func newTestAuthService(repo repository.UserRepository, store SessionStore, rotation bool) AuthService {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: rotation, TokenVersionCacheTTL: time.Minute}}
	return NewAuthService(AuthServiceParams{Repo: repo, Sessions: store, JWT: commonjwt.NewService(cfg.Auth), Config: cfg})
}

type authRepoStub struct {
	userByEmail       *ent.User
	gotEmail          string
	newVersion        int64
	incrementedUserID int64
}

func (r *authRepoStub) Create(context.Context, repository.CreateUserInput) (*ent.User, error) {
	return nil, nil
}
func (r *authRepoStub) ExistsByEmail(context.Context, string) (bool, error) { return false, nil }
func (r *authRepoStub) GetByID(context.Context, int64) (*ent.User, error)   { return nil, nil }
func (r *authRepoStub) ListUsers(context.Context, repository.ListUsersInput) ([]*ent.User, int, error) {
	return nil, 0, nil
}
func (r *authRepoStub) GetByEmail(_ context.Context, email string) (*ent.User, error) {
	r.gotEmail = email
	if r.userByEmail == nil {
		return nil, response.NotFoundError("user not found")
	}
	return r.userByEmail, nil
}
func (r *authRepoStub) GetTokenVersion(context.Context, int64) (int64, error) { return 0, nil }
func (r *authRepoStub) IncrementTokenVersion(_ context.Context, id int64) (int64, error) {
	r.incrementedUserID = id
	return r.newVersion, nil
}

type sessionStoreStub struct {
	version          int64
	session          Session
	created          Session
	deleted          bool
	deletedSessionID string
	deletedAll       bool
	invalidated      bool
}

func (s *sessionStoreStub) GetCurrentTokenVersion(context.Context, int64) (int64, error) {
	return s.version, nil
}
func (s *sessionStoreStub) ValidateTokenVersion(context.Context, string, int64) error { return nil }
func (s *sessionStoreStub) CreateSession(_ context.Context, session Session, _ time.Duration) error {
	s.created = session
	return nil
}
func (s *sessionStoreStub) GetSession(context.Context, string) (Session, error) {
	if s.session.SessionID == "" {
		return Session{}, ErrSessionNotFound
	}
	return s.session, nil
}
func (s *sessionStoreStub) DeleteSession(_ context.Context, _ int64, sessionID string) error {
	if sessionID == "error" {
		return errors.New("delete failed")
	}
	s.deleted = true
	s.deletedSessionID = sessionID
	return nil
}
func (s *sessionStoreStub) DeleteAllUserSessions(context.Context, int64) error {
	s.deletedAll = true
	return nil
}
func (s *sessionStoreStub) InvalidateUserTokenVersion(context.Context, int64) error {
	s.invalidated = true
	return nil
}
