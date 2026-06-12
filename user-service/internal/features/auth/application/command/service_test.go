package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
)

var authTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestAuthUseCaseLogin(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)

	tokens, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.TokenType != commonauth.TokenTypeBearer || tokens.ExpiresIn != 900 {
		t.Fatalf("tokens = %#v", tokens)
	}
	if repo.gotUsername != "alice" {
		t.Fatalf("gotUsername = %q", repo.gotUsername)
	}
	if store.created.SessionID == "" || store.created.UserID != authTestUserID.String() || store.created.TokenVersion != 2 {
		t.Fatalf("created session = %#v", store.created)
	}
}

func TestAuthUseCaseLoginRejectsBlankTrimmedCredentials(t *testing.T) {
	svc := newTestAuthUseCases(&authRepoStub{}, &sessionStoreStub{}, true)

	_, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: " "})

	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want authdomain.ErrInvalidCredentials", err)
	}
}

func TestAuthUseCaseLoginUsesDefaultTTLs(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCasesWithConfig(repo, store, config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}})

	tokens, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

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

func TestAuthUseCaseLoginUsesExplicitTTLs(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	accessTTL := time.Minute
	refreshTTL := 2 * time.Hour
	svc := newTestAuthUseCasesWithConfig(repo, store, config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: accessTTL, RefreshTokenTTL: refreshTTL}})

	tokens, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

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

func TestAuthUseCaseLoginPassesMaxActiveSessionsPerUser(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCasesWithConfig(repo, store, config.AuthConfig{
		JWT:                      config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour},
		TokenVersionCacheTTL:     time.Minute,
		RefreshTokenRotation:     true,
		MaxActiveSessionsPerUser: 3,
	})

	_, err = svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if store.createdMaxActiveSessionsPerUser != 3 {
		t.Fatalf("created limit = %#v, want max 3", store.createdMaxActiveSessionsPerUser)
	}
}

func TestAuthUseCaseLoginDoesNotReturnTokenWhenSessionCreateFails(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	createErr := errors.New("create session failed")
	repo := &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusNormal, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2, createErr: createErr}
	svc := newTestAuthUseCases(repo, store, true)

	tokens, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

	if !errors.Is(err, createErr) {
		t.Fatalf("Login err = %v, want create session error", err)
	}
	if tokens != nil {
		t.Fatalf("tokens = %#v, want nil", tokens)
	}
}

func TestAuthUseCaseLoginRejectsInvalidCredentials(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	svc := newTestAuthUseCases(&authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusNormal, TokenVersion: 2}}, &sessionStoreStub{version: 2}, true)

	_, err = svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "wrong"})

	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want authdomain.ErrInvalidCredentials", err)
	}
}

func TestAuthUseCaseLoginRejectsInactiveStatuses(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	for _, status := range []userdomain.UserStatus{userdomain.UserStatusDisabled} {
		svc := newTestAuthUseCases(&authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: status, TokenVersion: 2}}, &sessionStoreStub{version: 2}, true)

		_, err = svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

		if !errors.Is(err, authdomain.ErrInvalidCredentials) {
			t.Fatalf("status %d err = %v, want authdomain.ErrInvalidCredentials", status, err)
		}
	}
}

func TestAuthUseCaseLoginIssuesPasswordChangeToken(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)

	tokens, err := svc.Login(context.Background(), LoginCommand{Username: "alice", Password: "secret"})

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
	if claims.UserID != authTestUserID.String() || claims.TokenVersion != 2 || claims.Subject != commonauth.SubjectPasswordChange {
		t.Fatalf("claims = %#v", claims)
	}
	if store.created.SessionID != "" {
		t.Fatalf("created normal session = %#v", store.created)
	}
}

func TestAuthUseCaseChangePassword(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)
	token, err := testJWTService().SignPasswordChangeToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	result, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{Token: token, NewPassword: "new-secret"})

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if !result.Changed || repo.updatedInput.UserID != authTestUserID || repo.updatedInput.Status != userdomain.UserStatusNormal || repo.incrementedUserID != uuid.Nil || !store.cached || store.cachedVersion != 3 || !store.deletedAll {
		t.Fatalf("result=%#v repo=%#v store=%#v", result, repo, store)
	}
	matched, err := password.VerifyContext(context.Background(), "new-secret", repo.updatedInput.PasswordHash)
	if err != nil || !matched {
		t.Fatalf("updated password hash mismatch: matched=%v err=%v", matched, err)
	}
}

func TestAuthUseCaseChangePasswordIncrementsTokenVersionOnce(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)
	token, err := testJWTService().SignPasswordChangeToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	_, err = svc.ChangePassword(context.Background(), ChangePasswordCommand{Token: token, NewPassword: "new-secret"})

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("UpdateCredentials calls = %d, want 1", repo.updateCalls)
	}
	if repo.incrementCalls != 0 || repo.incrementedUserID != uuid.Nil {
		t.Fatalf("IncrementTokenVersion calls = %d userID=%s, want none", repo.incrementCalls, repo.incrementedUserID)
	}
	if !store.cached || store.cachedVersion != 3 || !store.deletedAll {
		t.Fatalf("store projection = %#v", store)
	}
}

func TestAuthUseCaseChangePasswordSucceedsWhenRevocationProjectionFails(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	store := &sessionStoreStub{version: 2, cacheErr: errors.New("cache failed"), deleteAllErr: errors.New("delete failed")}
	svc := newTestAuthUseCases(repo, store, true)
	token, err := testJWTService().SignPasswordChangeToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	result, err := svc.ChangePassword(context.Background(), ChangePasswordCommand{Token: token, NewPassword: "new-secret"})

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if result == nil || !result.Changed {
		t.Fatalf("result = %#v", result)
	}
	if !store.cacheDeleted || store.deletedCachedUserID != authTestUserID.String() {
		t.Fatalf("cache eviction = %#v", store)
	}
	if repo.incrementCalls != 0 {
		t.Fatalf("IncrementTokenVersion calls = %d, want 0", repo.incrementCalls)
	}
}

func TestAuthUseCaseChangePasswordMapsCredentialUpdateNotFound(t *testing.T) {
	repo := &authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, updateErr: userdomain.ErrUserNotFound}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)
	token, err := testJWTService().SignPasswordChangeToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	_, err = svc.ChangePassword(context.Background(), ChangePasswordCommand{Token: token, NewPassword: "new-secret"})

	if !errors.Is(err, userdomain.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAuthUseCaseChangePasswordMapsTokenVersionUserNotFound(t *testing.T) {
	svc := newTestAuthUseCases(&authRepoStub{tokenVersionErr: userdomain.ErrUserNotFound}, &sessionStoreStub{cacheMiss: true}, true)
	token, err := testJWTService().SignPasswordChangeToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}

	_, err = svc.ChangePassword(context.Background(), ChangePasswordCommand{Token: token, NewPassword: "new-secret"})

	if !errors.Is(err, userdomain.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAuthUseCaseChangePasswordRejectsAccessToken(t *testing.T) {
	repo := &authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)
	token, err := testJWTService().SignAccessToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	_, err = svc.ChangePassword(context.Background(), ChangePasswordCommand{Token: token, NewPassword: "new-secret"})

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestAuthUseCaseRefreshRotatesSession(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthUseCases(&authRepoStub{}, store, true)
	refresh, err := testJWTService().SignRefreshToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	tokens, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: " " + commonauth.TokenPrefix + refresh + " "})

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

func TestAuthUseCaseRefreshRotationPassesMaxActiveSessionsPerUser(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthUseCasesWithConfig(&authRepoStub{}, store, config.AuthConfig{
		JWT:                      config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour},
		TokenVersionCacheTTL:     time.Minute,
		RefreshTokenRotation:     true,
		MaxActiveSessionsPerUser: 4,
	})
	refresh, err := testJWTService().SignRefreshToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: refresh})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if store.rotatedMaxActiveSessionsPerUser != 4 {
		t.Fatalf("rotated limit = %#v, want max 4", store.rotatedMaxActiveSessionsPerUser)
	}
}

func TestAuthUseCaseRefreshRejectsInvalidNormalizedToken(t *testing.T) {
	svc := newTestAuthUseCases(&authRepoStub{}, &sessionStoreStub{}, false)

	for _, token := range []string{"", " ", commonauth.TokenTypeBearer, commonauth.TokenPrefix} {
		_, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: token})

		if !errors.Is(err, authdomain.ErrTokenInvalid) {
			t.Fatalf("token %q err = %v, want authdomain.ErrTokenInvalid", token, err)
		}
	}
}

func TestAuthUseCaseRefreshRotationKeepsOldSessionWhenTokenSigningFails(t *testing.T) {
	sessions := newRefreshRotationSessionLifecycle()
	svc := &refreshTokenUseCase{deps: &UseCaseDeps{tokens: &refreshRotationTokenIssuer{issueErr: errors.New("sign failed")}, sessions: sessions, refreshTokenRotation: true}}

	_, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if err == nil {
		t.Fatal("Refresh error is nil")
	}
	if sessions.createdSessionID != "" {
		t.Fatalf("created session = %q, want none", sessions.createdSessionID)
	}
	if len(sessions.deletedSessionIDs) != 0 {
		t.Fatalf("deleted sessions = %v, want none", sessions.deletedSessionIDs)
	}
}

func TestAuthUseCaseRefreshRotationKeepsOldSessionWhenNewSessionCreateFails(t *testing.T) {
	sessions := newRefreshRotationSessionLifecycle()
	sessions.createErr = errors.New("create failed")
	svc := &refreshTokenUseCase{deps: &UseCaseDeps{tokens: &refreshRotationTokenIssuer{}, sessions: sessions, refreshTokenRotation: true}}

	tokens, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if err == nil {
		t.Fatal("Refresh error is nil")
	}
	if tokens != nil {
		t.Fatalf("tokens = %#v, want nil", tokens)
	}
	if len(sessions.deletedSessionIDs) != 0 {
		t.Fatalf("deleted sessions = %v, want none", sessions.deletedSessionIDs)
	}
}

func TestAuthUseCaseRefreshRotationFailureDoesNotReturnToken(t *testing.T) {
	sessions := newRefreshRotationSessionLifecycle()
	sessions.rotateErr = errors.New("rotate failed")
	svc := &refreshTokenUseCase{deps: &UseCaseDeps{tokens: &refreshRotationTokenIssuer{}, sessions: sessions, refreshTokenRotation: true}}

	tokens, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if err == nil {
		t.Fatal("Refresh error is nil")
	}
	if tokens != nil {
		t.Fatalf("tokens = %#v, want nil", tokens)
	}
	if sessions.createdSessionID != "" {
		t.Fatalf("created session = %q, want none after atomic rotation failure", sessions.createdSessionID)
	}
	if len(sessions.deletedSessionIDs) != 0 {
		t.Fatalf("deleted sessions = %v, want none after atomic rotation failure", sessions.deletedSessionIDs)
	}
}

func TestAuthUseCaseRefreshRotationReturnsTokenAfterNewSessionAndOldRevocation(t *testing.T) {
	sessions := newRefreshRotationSessionLifecycle()
	svc := &refreshTokenUseCase{deps: &UseCaseDeps{tokens: &refreshRotationTokenIssuer{}, sessions: sessions, refreshTokenRotation: true}}

	tokens, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: "refresh"})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens == nil || tokens.AccessToken != "access" || tokens.RefreshToken != "refresh-new" {
		t.Fatalf("tokens = %#v", tokens)
	}
	if sessions.createdSessionID == "" || sessions.createdSessionID == "s-old" {
		t.Fatalf("created session = %q", sessions.createdSessionID)
	}
	if len(sessions.deletedSessionIDs) != 1 || sessions.deletedSessionIDs[0] != "s-old" {
		t.Fatalf("deleted sessions = %v, want old session only", sessions.deletedSessionIDs)
	}
}

func TestAuthUseCaseRefreshUsesNormalizedToken(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthUseCases(&authRepoStub{}, store, false)
	refresh, err := testJWTService().SignRefreshToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	tokens, err := svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: refresh})

	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestAuthUseCaseRefreshRejectsAccessTokenSubject(t *testing.T) {
	store := &sessionStoreStub{version: 2, session: authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthUseCases(&authRepoStub{}, store, false)
	access, err := testJWTService().SignAccessToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: access})

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestAuthUseCaseRefreshRejectsVersionChange(t *testing.T) {
	store := &sessionStoreStub{version: 3, session: authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}}
	svc := newTestAuthUseCases(&authRepoStub{}, store, true)
	refresh, err := testJWTService().SignRefreshToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: refresh})

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestAuthUseCaseRefreshMapsTokenVersionUserNotFound(t *testing.T) {
	store := &sessionStoreStub{session: authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}, cacheMiss: true}
	svc := newTestAuthUseCases(&authRepoStub{tokenVersionErr: userdomain.ErrUserNotFound}, store, true)
	refresh, err := testJWTService().SignRefreshToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	_, err = svc.Refresh(context.Background(), RefreshTokenCommand{RefreshToken: refresh})

	if !errors.Is(err, userdomain.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestAuthUseCaseLogoutAllIncrementsVersionAndDeletesSessions(t *testing.T) {
	repo := &authRepoStub{newVersion: 3}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	result, err := svc.LogoutAllSessions(ctx)

	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if !result.LoggedOut || repo.incrementedUserID != authTestUserID || !store.cached || store.cachedVersion != 3 || !store.deletedAll {
		t.Fatalf("result=%#v repo=%#v store=%#v", result, repo, store)
	}
}

func TestAuthUseCaseLogoutAllSucceedsWhenRevocationProjectionFails(t *testing.T) {
	repo := &authRepoStub{newVersion: 3}
	store := &sessionStoreStub{version: 2, cacheErr: errors.New("cache failed"), deleteAllErr: errors.New("delete failed")}
	svc := newTestAuthUseCases(repo, store, true)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	result, err := svc.LogoutAllSessions(ctx)

	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if result == nil || !result.LoggedOut {
		t.Fatalf("result = %#v", result)
	}
	if repo.incrementCalls != 1 || repo.incrementedUserID != authTestUserID {
		t.Fatalf("repo increment calls=%d userID=%s", repo.incrementCalls, repo.incrementedUserID)
	}
	if !store.cacheDeleted || store.deletedCachedUserID != authTestUserID.String() {
		t.Fatalf("cache eviction = %#v", store)
	}
}

func TestAuthUseCaseLogoutAllMapsIncrementUserNotFound(t *testing.T) {
	repo := &authRepoStub{incrementErr: userdomain.ErrUserNotFound}
	store := &sessionStoreStub{version: 2}
	svc := newTestAuthUseCases(repo, store, true)
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), authTestUserID.String()), "s-123")

	_, err := svc.LogoutAllSessions(ctx)

	if !errors.Is(err, userdomain.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
	if store.cached || store.deletedAll {
		t.Fatalf("sessions mutated after increment failure: %#v", store)
	}
}

func newTestAuthUseCases(repo *authRepoStub, store authapplication.AuthSessionStore, rotation bool) testAuthUseCases {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}, RefreshTokenRotation: rotation, TokenVersionCacheTTL: time.Minute, MaxActiveSessionsPerUser: 5}}
	return newTestAuthUseCasesWithConfig(repo, store, cfg.Auth)
}

func newTestAuthUseCasesWithConfig(repo *authRepoStub, store authapplication.AuthSessionStore, authCfg config.AuthConfig) testAuthUseCases {
	cfg := &config.Config{Auth: authCfg}
	deps := NewUseCaseDeps(UseCaseDepsParams{Credentials: repo, TokenVersions: repo, Sessions: store, JWT: commonauth.NewJWTService(cfg.Auth), Config: cfg})
	return testAuthUseCases{
		LoginUseCase:                NewLoginUseCase(deps),
		RefreshTokenUseCase:         NewRefreshTokenUseCase(deps),
		ChangePasswordUseCase:       NewChangePasswordUseCase(deps),
		LogoutCurrentSessionUseCase: NewLogoutCurrentSessionUseCase(deps),
		LogoutAllSessionsUseCase:    NewLogoutAllSessionsUseCase(deps),
	}
}

type testAuthUseCases struct {
	LoginUseCase
	RefreshTokenUseCase
	ChangePasswordUseCase
	LogoutCurrentSessionUseCase
	LogoutAllSessionsUseCase
}

func testJWTService() *commonauth.JWTService {
	return commonauth.NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}})
}

type authRepoStub struct {
	userByUsername    *authdomain.UserCredential
	userByID          *authdomain.UserCredential
	gotUsername       string
	newVersion        int64
	incrementErr      error
	updateErr         error
	incrementedUserID uuid.UUID
	incrementCalls    int
	updatedInput      authdomain.UpdateCredentialsInput
	updateCalls       int
	tokenVersion      int64
	tokenVersionErr   error
	getTokenVersionID uuid.UUID
}

func (r *authRepoStub) GetCredentialByUserID(_ context.Context, _ uuid.UUID) (*authdomain.UserCredential, error) {
	if r.userByID == nil {
		return nil, userdomain.ErrUserNotFound
	}
	return r.userByID, nil
}
func (r *authRepoStub) GetByUsername(_ context.Context, username string) (*authdomain.UserCredential, error) {
	r.gotUsername = username
	if r.userByUsername == nil {
		return nil, userdomain.ErrUserNotFound
	}
	return r.userByUsername, nil
}
func (r *authRepoStub) GetTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	r.getTokenVersionID = userID
	if r.tokenVersionErr != nil {
		return 0, r.tokenVersionErr
	}
	return r.tokenVersion, nil
}
func (r *authRepoStub) IncrementTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	r.incrementCalls++
	r.incrementedUserID = userID
	if r.incrementErr != nil {
		return 0, r.incrementErr
	}
	return r.newVersion, nil
}
func (r *authRepoStub) UpdateCredentials(_ context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
	r.updateCalls++
	r.updatedInput = input
	if r.updateErr != nil {
		return 0, r.updateErr
	}
	return r.newVersion, nil
}

type sessionStoreStub struct {
	version                         int64
	session                         authdomain.AuthSession
	created                         authdomain.AuthSession
	createdTTL                      time.Duration
	deleted                         bool
	deletedSessionID                string
	deletedAll                      bool
	getVersionErr                   error
	deleteAllErr                    error
	rotateErr                       error
	cacheMiss                       bool
	cacheErr                        error
	deleteCacheErr                  error
	cacheDeleted                    bool
	deletedCachedUserID             string
	cached                          bool
	cachedUserID                    string
	cachedVersion                   int64
	createdMaxActiveSessionsPerUser int
	rotatedMaxActiveSessionsPerUser int
	createErr                       error
}

func (s *sessionStoreStub) GetCachedTokenVersion(context.Context, string) (int64, error) {
	if s.getVersionErr != nil {
		return 0, s.getVersionErr
	}
	if s.cacheMiss {
		return 0, authdomain.ErrTokenVersionCacheMiss
	}
	return s.version, nil
}
func (s *sessionStoreStub) CacheTokenVersion(_ context.Context, userID string, tokenVersion int64) error {
	if s.cacheErr != nil {
		return s.cacheErr
	}
	s.cached = true
	s.cachedUserID = userID
	s.cachedVersion = tokenVersion
	return nil
}
func (s *sessionStoreStub) DeleteCachedTokenVersion(_ context.Context, userID string) error {
	if s.deleteCacheErr != nil {
		return s.deleteCacheErr
	}
	s.cacheDeleted = true
	s.deletedCachedUserID = userID
	return nil
}
func (s *sessionStoreStub) CreateSession(_ context.Context, session authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = session
	s.createdTTL = ttl
	s.createdMaxActiveSessionsPerUser = maxActiveSessionsPerUser
	return nil
}
func (s *sessionStoreStub) RotateSession(_ context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error {
	if s.rotateErr != nil {
		return s.rotateErr
	}
	s.deleted = true
	s.deletedSessionID = oldSession.SessionID
	s.created = newSession
	s.createdTTL = ttl
	s.rotatedMaxActiveSessionsPerUser = maxActiveSessionsPerUser
	return nil
}
func (s *sessionStoreStub) GetSession(context.Context, string, string) (authdomain.AuthSession, error) {
	if s.session.SessionID == "" {
		return authdomain.AuthSession{}, authdomain.ErrAuthSessionNotFound
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

type refreshRotationTokenIssuer struct {
	issueErr error
}

func (i *refreshRotationTokenIssuer) IssueTokenPair(context.Context, string, int64, string) (*issuedTokenPair, error) {
	if i.issueErr != nil {
		return nil, i.issueErr
	}
	return &issuedTokenPair{
		Response:   &TokenResult{AccessToken: "access", RefreshToken: "refresh-new", TokenType: commonauth.TokenTypeBearer, ExpiresIn: 900},
		RefreshTTL: time.Hour,
	}, nil
}

func (i *refreshRotationTokenIssuer) IssuePasswordChangeToken(context.Context, string, int64, string) (*TokenResult, error) {
	return nil, errors.New("not implemented")
}

func (i *refreshRotationTokenIssuer) ParseRefreshToken(context.Context, string) (*commonauth.Claims, error) {
	return &commonauth.Claims{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-old"}, nil
}

func (i *refreshRotationTokenIssuer) ParsePasswordChangeToken(context.Context, string) (*commonauth.Claims, uuid.UUID, error) {
	return nil, uuid.Nil, errors.New("not implemented")
}

type refreshRotationSessionLifecycle struct {
	createErr            error
	rotateErr            error
	deleteErrBySessionID map[string]error
	createdSessionID     string
	deletedSessionIDs    []string
}

func newRefreshRotationSessionLifecycle() *refreshRotationSessionLifecycle {
	return &refreshRotationSessionLifecycle{deleteErrBySessionID: map[string]error{}}
}

func (m *refreshRotationSessionLifecycle) CreateTokenSession(_ context.Context, _ string, sessionID string, _ int64, _ time.Duration) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.createdSessionID = sessionID
	return nil
}

func (m *refreshRotationSessionLifecycle) ValidatePasswordChangeClaims(context.Context, *commonauth.Claims) error {
	return errors.New("not implemented")
}

func (m *refreshRotationSessionLifecycle) ValidateRefreshSession(context.Context, *commonauth.Claims) (authdomain.AuthSession, int64, error) {
	return authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: "s-old", TokenVersion: 2}, 2, nil
}

func (m *refreshRotationSessionLifecycle) RotateTokenSession(_ context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, _ time.Duration) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.rotateErr != nil {
		return m.rotateErr
	}
	m.createdSessionID = newSession.SessionID
	m.deletedSessionIDs = append(m.deletedSessionIDs, oldSession.SessionID)
	return m.deleteErrBySessionID[oldSession.SessionID]
}

func (m *refreshRotationSessionLifecycle) DeleteSession(_ context.Context, _ string, sessionID string) error {
	m.deletedSessionIDs = append(m.deletedSessionIDs, sessionID)
	return m.deleteErrBySessionID[sessionID]
}

func (m *refreshRotationSessionLifecycle) RevokeUserSessionsAtVersion(context.Context, uuid.UUID, int64) error {
	return errors.New("not implemented")
}

func (m *refreshRotationSessionLifecycle) RevokeAllUserSessions(context.Context, uuid.UUID) (*authdomain.SessionRevocationResult, error) {
	return nil, errors.New("not implemented")
}
