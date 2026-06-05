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
	"github.com/aegiscore/user-services/internal/repository"
)

func TestCredentialVerifierAcceptsMustChangePasswordUser(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verifier := newCredentialVerifier(&authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusMustChangePassword, TokenVersion: 2}})

	user, err := verifier.VerifyPassword(context.Background(), "alice", "secret")

	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !user.RequiresPasswordChange() {
		t.Fatalf("user status = %d, want must change password", user.Status)
	}
}

func TestCredentialVerifierRejectsDisabledUser(t *testing.T) {
	passwordHash, err := password.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verifier := newCredentialVerifier(&authRepoStub{userByUsername: &domain.User{ID: 123, UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: domain.UserStatusDisabled, TokenVersion: 2}})

	_, err = verifier.VerifyPassword(context.Background(), "alice", "secret")

	appErr := response.FromError(err)
	if appErr.Code != response.CodeUnauthenticated {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestCredentialVerifierChangePasswordUpdatesCredentials(t *testing.T) {
	oldHash, err := password.Hash("old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByID: &domain.User{ID: 123, UserID: authTestUserID, PasswordHash: oldHash, Status: domain.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	verifier := newCredentialVerifier(repo)

	result, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if result.UserID != authTestUserID || result.TokenVersion != 3 {
		t.Fatalf("result = %#v", result)
	}
	if repo.updatedInput.UserID != authTestUserID || repo.updatedInput.Status != domain.UserStatusNormal {
		t.Fatalf("updated input = %#v", repo.updatedInput)
	}
	matched, err := password.Verify("new-secret", repo.updatedInput.PasswordHash)
	if err != nil || !matched {
		t.Fatalf("updated password hash mismatch: matched=%v err=%v", matched, err)
	}
}

func TestCredentialVerifierChangePasswordMapsUserNotFound(t *testing.T) {
	verifier := newCredentialVerifier(&authRepoStub{})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	appErr := response.FromError(err)
	if appErr.Code != response.CodeNotFound {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestCredentialVerifierChangePasswordRejectsInvalidStatus(t *testing.T) {
	verifier := newCredentialVerifier(&authRepoStub{userByID: &domain.User{ID: 123, UserID: authTestUserID, Status: domain.UserStatusNormal, TokenVersion: 2}})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestCredentialVerifierChangePasswordMapsUpdateError(t *testing.T) {
	verifier := newCredentialVerifier(&authRepoStub{userByID: &domain.User{ID: 123, UserID: authTestUserID, Status: domain.UserStatusMustChangePassword, TokenVersion: 2}, updateErr: errors.New("update failed")})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	appErr := response.FromError(err)
	if appErr.Code != response.CodeInternalError {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthTokenIssuerParsesBearerRefreshToken(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}}}
	jwt := auth.NewJWTService(cfg.Auth)
	issuer := newAuthTokenIssuer(jwt, cfg)
	refresh, err := jwt.SignRefreshToken(auth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	claims, err := issuer.ParseRefreshToken(context.Background(), "Bearer "+refresh)

	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if claims.UserID != authTestUserID.String() || claims.SessionID != "s-123" || claims.Subject != auth.SubjectRefresh {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestAuthSessionManagerRejectsRefreshVersionMismatch(t *testing.T) {
	manager := newAuthSessionManager(&authRepoStub{}, &sessionStoreStub{version: 3, session: authRefreshTestSession("s-123", 2)})
	claims := &auth.Claims{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123"}

	_, _, err := manager.ValidateRefreshSession(context.Background(), claims)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthSessionManagerRevokeAllUserSessions(t *testing.T) {
	repo := &authRepoStub{newVersion: 4}
	store := &sessionStoreStub{}
	manager := newAuthSessionManager(repo, store)

	result, err := manager.RevokeAllUserSessions(context.Background(), authTestUserID)

	if err != nil {
		t.Fatalf("RevokeAllUserSessions: %v", err)
	}
	if result.UserID != authTestUserID || result.TokenVersion != 4 {
		t.Fatalf("result = %#v", result)
	}
	if repo.incrementedUserID != authTestUserID || !store.invalidated || !store.deletedAll {
		t.Fatalf("repo=%#v store=%#v", repo, store)
	}
}

func TestAuthSessionManagerRevokeAllUserSessionsMapsUserNotFound(t *testing.T) {
	store := &sessionStoreStub{}
	manager := newAuthSessionManager(&authRepoStub{incrementErr: domain.ErrUserNotFound}, store)

	_, err := manager.RevokeAllUserSessions(context.Background(), authTestUserID)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeNotFound {
		t.Fatalf("err = %#v", appErr)
	}
	if store.invalidated || store.deletedAll {
		t.Fatalf("store mutated after increment failure: %#v", store)
	}
}

func TestAuthSessionManagerRevokeAllUserSessionsStopsOnInvalidateError(t *testing.T) {
	store := &sessionStoreStub{invalidateErr: errors.New("invalidate failed")}
	manager := newAuthSessionManager(&authRepoStub{newVersion: 4}, store)

	_, err := manager.RevokeAllUserSessions(context.Background(), authTestUserID)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeInternalError {
		t.Fatalf("err = %#v", appErr)
	}
	if store.deletedAll {
		t.Fatalf("deleted all after invalidate failure: %#v", store)
	}
}

func TestAuthSessionManagerRevokeAllUserSessionsMapsDeleteAllError(t *testing.T) {
	store := &sessionStoreStub{deleteAllErr: errors.New("delete all failed")}
	manager := newAuthSessionManager(&authRepoStub{newVersion: 4}, store)

	_, err := manager.RevokeAllUserSessions(context.Background(), authTestUserID)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeInternalError {
		t.Fatalf("err = %#v", appErr)
	}
	if !store.invalidated {
		t.Fatalf("token version cache was not invalidated before delete failure")
	}
}

func authRefreshTestSession(sessionID string, tokenVersion int64) repository.AuthSession {
	return repository.AuthSession{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}
