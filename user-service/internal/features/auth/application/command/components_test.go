package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
)

func TestCredentialVerifierAcceptsMustChangePasswordUser(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verifier := authcredentials.NewVerifier(&authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}})

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
	verifier := authcredentials.NewVerifier(&authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusDisabled, TokenVersion: 2}})

	_, err = verifier.VerifyPassword(context.Background(), "alice", "secret")

	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want authdomain.ErrInvalidCredentials", err)
	}
}

func TestCredentialVerifierLoginFailureLogsClientContext(t *testing.T) {
	passwordHash, err := password.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	tests := []struct {
		name       string
		repo       *authRepoStub
		username   string
		password   string
		message    string
		wantUserID bool
		wantStatus bool
	}{
		{name: "user not found", repo: &authRepoStub{}, username: "alice", password: "secret", message: "login user not found"},
		{name: "password mismatch", repo: &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusNormal, TokenVersion: 2}}, username: "alice", password: "wrong", message: "login password mismatch", wantUserID: true},
		{name: "status rejected", repo: &authRepoStub{userByUsername: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: userdomain.UserStatusDisabled, TokenVersion: 2}}, username: "alice", password: "secret", message: "login user status rejected", wantUserID: true, wantStatus: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			ctx := logger.ToContext(context.Background(), zap.New(core))
			ctx = authctx.WithClientContext(ctx, authctx.ClientContext{ClientIP: "203.0.113.30", UserAgent: "auth-command-test"})
			verifier := authcredentials.NewVerifier(tt.repo)

			_, err := verifier.VerifyPassword(ctx, tt.username, tt.password)

			if !errors.Is(err, authdomain.ErrInvalidCredentials) {
				t.Fatalf("err = %v, want authdomain.ErrInvalidCredentials", err)
			}
			entries := logs.FilterMessage(tt.message).All()
			if len(entries) != 1 {
				t.Fatalf("log count = %d, want 1", len(entries))
			}
			fields := entries[0].ContextMap()
			if fields["username"] != tt.username || fields["client_ip"] != "203.0.113.30" || fields["user_agent"] != "auth-command-test" {
				t.Fatalf("log fields = %#v", fields)
			}
			if tt.wantUserID && fields["user_id"] != authTestUserID.String() {
				t.Fatalf("user_id = %#v, want %s; fields = %#v", fields["user_id"], authTestUserID.String(), fields)
			}
			if tt.wantStatus && fields["status"] != int64(userdomain.UserStatusDisabled) {
				t.Fatalf("status = %#v, want %d; fields = %#v", fields["status"], userdomain.UserStatusDisabled, fields)
			}
		})
	}
}

func TestCredentialVerifierChangePasswordUpdatesCredentials(t *testing.T) {
	oldHash, err := password.HashContext(context.Background(), "old-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	repo := &authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, PasswordHash: oldHash, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, newVersion: 3}
	verifier := authcredentials.NewVerifier(repo)

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
	verifier := authcredentials.NewVerifier(&authRepoStub{})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	if !errors.Is(err, userdomain.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestCredentialVerifierChangePasswordRejectsInvalidStatus(t *testing.T) {
	verifier := authcredentials.NewVerifier(&authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Status: userdomain.UserStatusNormal, TokenVersion: 2}})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestCredentialVerifierChangePasswordMapsUpdateError(t *testing.T) {
	updateErr := errors.New("update failed")
	verifier := authcredentials.NewVerifier(&authRepoStub{userByID: &authdomain.UserCredential{UserID: authTestUserID, Status: userdomain.UserStatusMustChangePassword, TokenVersion: 2}, updateErr: updateErr})

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, "new-secret")

	if !errors.Is(err, updateErr) {
		t.Fatalf("err = %v, want %v", err, updateErr)
	}
}

func TestAuthTokenIssuerParsesBearerRefreshToken(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}}}
	jwt := commonauth.NewJWTService(cfg.Auth)
	issuer := authtokens.NewIssuer(jwt, cfg)
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
	lifecycle := newTestAuthSessionLifecycle(&authRepoStub{}, &sessionStoreStub{version: 3, session: authRefreshTestSession("s-123", 2)})
	claims := &commonauth.Claims{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123"}

	_, _, err := lifecycle.ValidateRefreshSession(context.Background(), claims)

	if !errors.Is(err, authdomain.ErrTokenInvalid) {
		t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
	}
}

func TestAuthSessionLifecycleRotateTokenSessionMapsRejectedSession(t *testing.T) {
	for _, err := range []error{authdomain.ErrAuthSessionNotFound, authdomain.ErrAuthSessionMismatch} {
		t.Run(err.Error(), func(t *testing.T) {
			lifecycle := newTestAuthSessionLifecycle(&authRepoStub{}, &sessionStoreStub{rotateErr: err})
			oldSession := authRefreshTestSession("s-old", 2)
			newSession := authRefreshTestSession("s-new", 2)

			err := lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)

			if !errors.Is(err, authdomain.ErrTokenInvalid) {
				t.Fatalf("err = %v, want authdomain.ErrTokenInvalid", err)
			}
		})
	}
}

func TestAuthSessionLifecycleRotateTokenSessionMapsUnexpectedError(t *testing.T) {
	rotateErr := errors.New("redis failed")
	lifecycle := newTestAuthSessionLifecycle(&authRepoStub{}, &sessionStoreStub{rotateErr: rotateErr})
	oldSession := authRefreshTestSession("s-old", 2)
	newSession := authRefreshTestSession("s-new", 2)

	err := lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)

	if !errors.Is(err, rotateErr) {
		t.Fatalf("err = %v, want %v", err, rotateErr)
	}
}

func TestAuthSessionLifecycleCurrentTokenVersionUsesCacheHit(t *testing.T) {
	repo := &authRepoStub{tokenVersionErr: errors.New("database should not be read")}
	lifecycle := newTestAuthSessionLifecycle(repo, &sessionStoreStub{version: 2})

	version, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())

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
	lifecycle := newTestAuthSessionLifecycle(repo, store)

	version, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())

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

func TestAuthSessionLifecycleCurrentTokenVersionCacheErrorReturnsInfrastructureError(t *testing.T) {
	repo := &authRepoStub{tokenVersion: 7}
	cacheErr := errors.New("redis failed")
	store := &sessionStoreStub{getVersionErr: cacheErr}
	lifecycle := newTestAuthSessionLifecycle(repo, store)

	_, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())

	if !errors.Is(err, cacheErr) {
		t.Fatalf("err = %v, want cache error", err)
	}
	if repo.getTokenVersionID != uuid.Nil {
		t.Fatalf("repository should not be read after cache infrastructure error: %s", repo.getTokenVersionID)
	}
	if store.cached {
		t.Fatalf("store = %#v", store)
	}
}

func TestAuthSessionLifecycleCurrentTokenVersionDatabaseFallbackErrorReturnsInfrastructureError(t *testing.T) {
	dbErr := errors.New("database failed")
	repo := &authRepoStub{tokenVersionErr: dbErr}
	store := &sessionStoreStub{cacheMiss: true}
	lifecycle := newTestAuthSessionLifecycle(repo, store)

	_, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())

	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want database error", err)
	}
	if repo.getTokenVersionID != authTestUserID {
		t.Fatalf("getTokenVersionID = %s, want %s", repo.getTokenVersionID, authTestUserID)
	}
	if store.cached {
		t.Fatalf("store should not be backfilled after database error: %#v", store)
	}
}

func TestAuthSessionLifecycleCurrentTokenVersionBackfillErrorReturnsInfrastructureError(t *testing.T) {
	cacheErr := errors.New("redis set failed")
	store := &sessionStoreStub{cacheMiss: true, cacheErr: cacheErr}
	lifecycle := newTestAuthSessionLifecycle(&authRepoStub{tokenVersion: 7}, store)

	_, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())

	if !errors.Is(err, cacheErr) {
		t.Fatalf("err = %v, want cache backfill error", err)
	}
}

func TestAuthSessionLifecycleRevokeAllUserSessions(t *testing.T) {
	repo := &authRepoStub{newVersion: 4}
	store := &sessionStoreStub{}
	lifecycle := newTestAuthSessionLifecycle(repo, store)

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
	lifecycle := newTestAuthSessionLifecycle(&authRepoStub{incrementErr: userdomain.ErrUserNotFound}, store)

	_, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)

	if !errors.Is(err, userdomain.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
	if store.cached || store.deletedAll {
		t.Fatalf("store mutated after increment failure: %#v", store)
	}
}

func TestAuthSessionLifecycleRevokeAllUserSessionsCompensatesCacheRefreshError(t *testing.T) {
	store := &sessionStoreStub{cacheErr: errors.New("cache refresh failed")}
	lifecycle := newTestAuthSessionLifecycle(&authRepoStub{newVersion: 4}, store)

	result, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)

	if err != nil {
		t.Fatalf("RevokeAllUserSessions: %v", err)
	}
	if result.UserID != authTestUserID || result.TokenVersion != 4 {
		t.Fatalf("result = %#v", result)
	}
	if !store.cacheDeleted || store.deletedCachedUserID != authTestUserID.String() || !store.deletedAll {
		t.Fatalf("store = %#v", store)
	}
}

func TestAuthSessionLifecycleRevokeAllUserSessionsSucceedsAfterDeleteAllProjectionError(t *testing.T) {
	store := &sessionStoreStub{deleteAllErr: errors.New("delete all failed")}
	lifecycle := newTestAuthSessionLifecycle(&authRepoStub{newVersion: 4}, store)

	result, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)

	if err != nil {
		t.Fatalf("RevokeAllUserSessions: %v", err)
	}
	if result.UserID != authTestUserID || result.TokenVersion != 4 {
		t.Fatalf("result = %#v", result)
	}
	if !store.cached || store.cachedVersion != 4 {
		t.Fatalf("token version cache was not refreshed before delete failure: %#v", store)
	}
}

func TestAuthSessionLifecycleRevokeUserSessionsAtVersionReturnsProjectionError(t *testing.T) {
	cacheErr := errors.New("cache refresh failed")
	deleteErr := errors.New("delete all failed")
	store := &sessionStoreStub{cacheErr: cacheErr, deleteAllErr: deleteErr}
	lifecycle := newTestAuthSessionLifecycle(&authRepoStub{newVersion: 99}, store)

	err := lifecycle.RevokeUserSessionsAtVersion(context.Background(), authTestUserID, 4)

	if !errors.Is(err, cacheErr) || !errors.Is(err, deleteErr) {
		t.Fatalf("err = %v, want cache and delete errors", err)
	}
	if !store.cacheDeleted || store.deletedCachedUserID != authTestUserID.String() {
		t.Fatalf("cache eviction = %#v", store)
	}
	if store.cached || store.cachedVersion != 0 {
		t.Fatalf("cache should not be marked refreshed after cache error: %#v", store)
	}
}

func TestTokenVersionValidatorRejectsStaleTokenWhenCacheHasNewVersion(t *testing.T) {
	validator := authvalidators.NewValidator(&authRepoStub{tokenVersionErr: errors.New("database should not be read")}, &sessionStoreStub{version: 4})

	err := validator.ValidateTokenVersion(context.Background(), authTestUserID.String(), 3)

	if !errors.Is(err, commonauth.ErrTokenVersionMismatch) {
		t.Fatalf("err = %v, want common token version mismatch", err)
	}
	var mismatch *commonauth.TokenVersionMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want structured token version mismatch", err)
	}
	if mismatch.Current != 4 || mismatch.Token != 3 {
		t.Fatalf("mismatch = %#v, want current=4 token=3", mismatch)
	}
}

func authRefreshTestSession(sessionID string, tokenVersion int64) authdomain.AuthSession {
	return authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}

func newTestAuthSessionLifecycle(users authapplication.UserTokenVersionStore, sessions authapplication.AuthSessionStore) authsessions.Lifecycle {
	return authsessions.NewLifecycle(users, sessions, 5)
}
