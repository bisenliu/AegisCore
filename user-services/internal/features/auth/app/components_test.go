package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	"github.com/google/uuid"
)

func TestCredentialVerifierAcceptsMustChangePasswordUser(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verifier := newCredentialVerifier(&authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}})

	user, err := verifier.VerifyPassword(context.Background(), "alice", "secret")

	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !user.RequiresPasswordChange() {
		t.Fatalf("user status = %d, want must change password", user.Status)
	}
}

func TestCredentialVerifierRejectsDisabledUser(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verifier := newCredentialVerifier(&authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusDisabled, TokenVersion: 2}})

	_, err = verifier.VerifyPassword(context.Background(), "alice", "secret")

	appErr := response.FromError(err)
	if appErr.Code != response.CodeUnauthenticated {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestCredentialVerifierChangePasswordUpdatesCredentials(t *testing.T) {
	oldHash, err := password.HashContext(context.Background(), "old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, PasswordHash: oldHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	verifier := newCredentialVerifier(repo)

	result, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if result.UserID != authTestUserID || result.TokenVersion != 3 {
		t.Fatalf("result = %#v", result)
	}
	if repo.updatedInput.UserID != authTestUserID || repo.updatedInput.Status != userdomain.UserStatusNormal {
		t.Fatalf("updated input = %#v", repo.updatedInput)
	}
	matched, err := password.VerifyContext(context.Background(), "new-secret", repo.updatedInput.PasswordHash)
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
	verifier := newCredentialVerifier(&authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Status: userdomain.UserStatusNormal, TokenVersion: 2}})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestCredentialVerifierChangePasswordMapsUpdateError(t *testing.T) {
	verifier := newCredentialVerifier(&authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, updateErr: errors.New("update failed")})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	appErr := response.FromError(err)
	if appErr.Code != response.CodeInternalError {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthTokenIssuerParsesBearerRefreshToken(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}}}
	jwt := commonauth.NewJWTService(cfg.Auth)
	issuer := newAuthTokenIssuer(jwt, cfg)
	refresh, err := jwt.SignRefreshToken(commonauth.SignInput{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	claims, err := issuer.ParseRefreshToken(context.Background(), "Bearer "+refresh)

	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if claims.UserID != authTestUserID.String() || claims.SessionID != "s-123" || claims.Subject != commonauth.SubjectRefresh {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestAuthSessionLifecycleRejectsRefreshVersionMismatch(t *testing.T) {
	lifecycle := newAuthSessionLifecycle(&authRepoStub{}, &sessionStoreStub{version: 3, session: authRefreshTestSession("s-123", 2)})
	claims := &commonauth.Claims{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123"}

	_, _, err := lifecycle.ValidateRefreshSession(context.Background(), claims)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func TestAuthSessionLifecycleRotateTokenSessionMapsRejectedSession(t *testing.T) {
	for _, err := range []error{authdomain.ErrAuthSessionNotFound, authdomain.ErrAuthSessionMismatch} {
		t.Run(err.Error(), func(t *testing.T) {
			lifecycle := newAuthSessionLifecycle(&authRepoStub{}, &sessionStoreStub{rotateErr: err})
			oldSession := authRefreshTestSession("s-old", 2)
			newSession := authRefreshTestSession("s-new", 2)

			err := lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)

			appErr := response.FromError(err)
			if appErr.Code != response.CodeTokenInvalid {
				t.Fatalf("err = %#v, want token invalid", appErr)
			}
		})
	}
}

func TestAuthSessionLifecycleRotateTokenSessionMapsUnexpectedError(t *testing.T) {
	lifecycle := newAuthSessionLifecycle(&authRepoStub{}, &sessionStoreStub{rotateErr: errors.New("redis failed")})
	oldSession := authRefreshTestSession("s-old", 2)
	newSession := authRefreshTestSession("s-new", 2)

	err := lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeInternalError {
		t.Fatalf("err = %#v, want internal", appErr)
	}
}

func TestAuthSessionLifecycleCurrentTokenVersionUsesCacheHit(t *testing.T) {
	repo := &authRepoStub{tokenVersionErr: errors.New("database should not be read")}
	lifecycle := newAuthSessionLifecycle(repo, &sessionStoreStub{version: 2})

	version, err := lifecycle.(*authSessionLifecycle).currentTokenVersion(context.Background(), authTestUserID.String())

	if err != nil {
		t.Fatalf("currentTokenVersion: %v", err)
	}
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
	if repo.getTokenVersionID != uuid.Nil {
		t.Fatalf("repository was read on cache hit: %s", repo.getTokenVersionID)
	}
}

func TestAuthSessionLifecycleCurrentTokenVersionCacheMissReadsRepository(t *testing.T) {
	repo := &authRepoStub{tokenVersion: 7}
	store := &sessionStoreStub{cacheMiss: true}
	lifecycle := newAuthSessionLifecycle(repo, store)

	version, err := lifecycle.(*authSessionLifecycle).currentTokenVersion(context.Background(), authTestUserID.String())

	if err != nil {
		t.Fatalf("currentTokenVersion: %v", err)
	}
	if version != 7 {
		t.Fatalf("version = %d, want 7", version)
	}
	if repo.getTokenVersionID != authTestUserID {
		t.Fatalf("getTokenVersionID = %s, want %s", repo.getTokenVersionID, authTestUserID)
	}
	if !store.cached || store.cachedUserID != authTestUserID.String() || store.cachedVersion != 7 {
		t.Fatalf("cached store = %#v", store)
	}
}

func TestAuthSessionLifecycleCurrentTokenVersionPropagatesCacheError(t *testing.T) {
	repo := &authRepoStub{tokenVersion: 7}
	cacheErr := errors.New("redis failed")
	lifecycle := newAuthSessionLifecycle(repo, &sessionStoreStub{getVersionErr: cacheErr})

	_, err := lifecycle.(*authSessionLifecycle).currentTokenVersion(context.Background(), authTestUserID.String())

	if !errors.Is(err, cacheErr) {
		t.Fatalf("err = %v, want %v", err, cacheErr)
	}
	if repo.getTokenVersionID != uuid.Nil {
		t.Fatalf("repository was read after cache error: %s", repo.getTokenVersionID)
	}
}

func TestAuthSessionLifecycleCurrentTokenVersionPropagatesBackfillError(t *testing.T) {
	cacheErr := errors.New("redis set failed")
	store := &sessionStoreStub{cacheMiss: true, cacheErr: cacheErr}
	lifecycle := newAuthSessionLifecycle(&authRepoStub{tokenVersion: 7}, store)

	_, err := lifecycle.(*authSessionLifecycle).currentTokenVersion(context.Background(), authTestUserID.String())

	if !errors.Is(err, cacheErr) {
		t.Fatalf("err = %v, want %v", err, cacheErr)
	}
}

func TestAuthSessionLifecycleRevokeAllUserSessions(t *testing.T) {
	repo := &authRepoStub{newVersion: 4}
	store := &sessionStoreStub{}
	lifecycle := newAuthSessionLifecycle(repo, store)

	result, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)

	if err != nil {
		t.Fatalf("RevokeAllUserSessions: %v", err)
	}
	if result.UserID != authTestUserID || result.TokenVersion != 4 {
		t.Fatalf("result = %#v", result)
	}
	if repo.incrementedUserID != authTestUserID || !store.cached || store.cachedVersion != 4 || !store.deletedAll {
		t.Fatalf("repo=%#v store=%#v", repo, store)
	}
}

func TestAuthSessionLifecycleRevokeAllUserSessionsMapsUserNotFound(t *testing.T) {
	store := &sessionStoreStub{}
	lifecycle := newAuthSessionLifecycle(&authRepoStub{incrementErr: userdomain.ErrUserNotFound}, store)

	_, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeNotFound {
		t.Fatalf("err = %#v", appErr)
	}
	if store.cached || store.deletedAll {
		t.Fatalf("store mutated after increment failure: %#v", store)
	}
}

func TestAuthSessionLifecycleRevokeAllUserSessionsStopsOnCacheRefreshError(t *testing.T) {
	store := &sessionStoreStub{cacheErr: errors.New("cache refresh failed")}
	lifecycle := newAuthSessionLifecycle(&authRepoStub{newVersion: 4}, store)

	_, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeInternalError {
		t.Fatalf("err = %#v", appErr)
	}
	if store.deletedAll {
		t.Fatalf("deleted all after cache refresh failure: %#v", store)
	}
}

func TestAuthSessionLifecycleRevokeAllUserSessionsMapsDeleteAllError(t *testing.T) {
	store := &sessionStoreStub{deleteAllErr: errors.New("delete all failed")}
	lifecycle := newAuthSessionLifecycle(&authRepoStub{newVersion: 4}, store)

	_, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeInternalError {
		t.Fatalf("err = %#v", appErr)
	}
	if !store.cached || store.cachedVersion != 4 {
		t.Fatalf("token version cache was not refreshed before delete failure: %#v", store)
	}
}

func TestTokenVersionValidatorRejectsStaleTokenWhenCacheHasNewVersion(t *testing.T) {
	validator := NewTokenVersionValidator(&authRepoStub{tokenVersionErr: errors.New("database should not be read")}, &sessionStoreStub{version: 4})

	err := validator.ValidateTokenVersion(context.Background(), authTestUserID.String(), 3)

	if !errors.Is(err, authdomain.ErrTokenVersionMismatch) {
		t.Fatalf("err = %v, want token version mismatch", err)
	}
}

func authRefreshTestSession(sessionID string, tokenVersion int64) authdomain.AuthSession {
	return authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}
