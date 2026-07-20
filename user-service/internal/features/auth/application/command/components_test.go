package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestCredentialVerifierAcceptsMustChangePasswordUser(t *testing.T) {
	passwordHash, err := hashTestPassword(t, "secret")
	require.NoError(t, err,
		"Hash: %v", err)

	ctrl := gomock.NewController(t)
	repo := NewMockUserCredentialStore(ctrl)
	verifier := authcredentials.NewVerifier(repo, testPasswordService(t))
	repo.EXPECT().GetByUsername(gomock.Any(), "alice").Return(&authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusMustChangePassword, TokenVersion: 2}, nil)

	user, err := verifier.VerifyPassword(context.Background(), "alice", "secret")
	require.NoError(t, err,
		"VerifyPassword: %v", err)
	require.True(t, user.RequiresPasswordChange(),
		"user status = %d, want must change password", user.Status)

}

func TestCredentialVerifierRejectsDisabledUser(t *testing.T) {
	passwordHash, err := hashTestPassword(t, "secret")
	require.NoError(t, err,
		"Hash: %v", err)

	ctrl := gomock.NewController(t)
	repo := NewMockUserCredentialStore(ctrl)
	verifier := authcredentials.NewVerifier(repo, testPasswordService(t))
	repo.EXPECT().GetByUsername(gomock.Any(), "alice").Return(&authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusDisabled, TokenVersion: 2}, nil)

	_, err = verifier.VerifyPassword(context.Background(), "alice", "secret")
	require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
		"err = %v, want authdomain.ErrInvalidCredentials", err)

}

func TestCredentialVerifierLoginFailureLogsClientContext(t *testing.T) {
	passwordHash, err := hashTestPassword(t, "secret")
	require.NoError(t, err,
		"Hash: %v", err)

	tests := []struct {
		name       string
		credential *authdomain.UserCredential
		repoErr    error
		username   string
		password   string
		message    string
		wantUserID bool
		wantStatus bool
	}{
		{name: "user not found", repoErr: identity.ErrUserNotFound, username: "alice", password: "secret", message: "login user not found"},
		{name: "password mismatch", credential: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusNormal, TokenVersion: 2}, username: "alice", password: "wrong", message: "login password mismatch", wantUserID: true},
		{name: "status rejected", credential: &authdomain.UserCredential{UserID: authTestUserID, Username: "alice", PasswordHash: passwordHash, Status: identity.UserStatusDisabled, TokenVersion: 2}, username: "alice", password: "secret", message: "login user status rejected", wantUserID: true, wantStatus: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			ctx := logger.ToContext(context.Background(), zap.New(core))
			ctx = authctx.WithClientContext(ctx, authctx.ClientContext{ClientIP: "203.0.113.30", UserAgent: "auth-command-test"})
			ctrl := gomock.NewController(t)
			repo := NewMockUserCredentialStore(ctrl)
			verifier := authcredentials.NewVerifier(repo, testPasswordService(t))
			repo.EXPECT().GetByUsername(gomock.Any(), tt.username).Return(tt.credential, tt.repoErr)

			_, err := verifier.VerifyPassword(ctx, tt.username, tt.password)
			require.ErrorIs(t, err, authdomain.ErrInvalidCredentials,
				"err = %v, want authdomain.ErrInvalidCredentials", err)

			entries := logs.FilterMessage(tt.message).All()
			require.EqualValues(t, 1, len(entries),
				"log count = %d, want 1", len(entries))

			fields := entries[0].ContextMap()
			require.False(t, fields["username"] != tt.username || fields["client_ip"] != "203.0.113.30" || fields["user_agent"] != "auth-command-test",
				"log fields = %#v", fields)
			require.False(t, tt.wantUserID && fields["user_id"] != authTestUserID.String(),
				"user_id = %#v, want %s; fields = %#v", fields["user_id"], authTestUserID.String(), fields)
			require.False(t, tt.wantStatus && fields["status"] != int64(identity.UserStatusDisabled),
				"status = %#v, want %d; fields = %#v", fields["status"], identity.UserStatusDisabled, fields)

		})
	}
}

func TestCredentialVerifierChangePasswordUpdatesCredentials(t *testing.T) {
	oldHash, err := hashTestPassword(t, "old-secret")
	require.NoError(t, err,
		"Hash: %v", err)

	ctrl := gomock.NewController(t)
	repo := NewMockUserCredentialStore(ctrl)
	verifier := authcredentials.NewVerifier(repo, testPasswordService(t))
	var updatedInput authdomain.UpdateCredentialsInput

	repo.EXPECT().GetCredentialByUserID(gomock.Any(), authTestUserID).Return(&authdomain.UserCredential{UserID: authTestUserID, PasswordHash: oldHash, Status: identity.UserStatusMustChangePassword, TokenVersion: 2}, nil)
	repo.EXPECT().UpdateCredentials(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
		updatedInput = input
		return 3, nil
	})

	result, err := verifier.ChangePassword(context.Background(), authTestUserID, 2, "new-secret")
	require.NoError(t, err,
		"ChangePassword: %v", err)
	require.False(t, result.UserID != authTestUserID || result.TokenVersion != 3,
		"result = %#v", result)
	require.False(t, updatedInput.UserID != authTestUserID || updatedInput.Status != identity.UserStatusNormal,
		"updated input = %#v", updatedInput)

	matched, err := verifyTestPassword(t, "new-secret", updatedInput.PasswordHash)
	require.False(t, err != nil || !matched,
		"updated password hash mismatch: matched=%v err=%v", matched, err)

}

func TestCredentialVerifierChangePasswordMapsUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockUserCredentialStore(ctrl)
	verifier := authcredentials.NewVerifier(repo, testPasswordService(t))
	repo.EXPECT().GetCredentialByUserID(gomock.Any(), authTestUserID).Return(nil, identity.ErrUserNotFound)

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, 2, "new-secret")
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}

func TestCredentialVerifierChangePasswordRejectsInvalidStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockUserCredentialStore(ctrl)
	verifier := authcredentials.NewVerifier(repo, testPasswordService(t))
	repo.EXPECT().GetCredentialByUserID(gomock.Any(), authTestUserID).Return(&authdomain.UserCredential{UserID: authTestUserID, Status: identity.UserStatusNormal, TokenVersion: 2}, nil)

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, 2, "new-secret")
	require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
		"err = %v, want authdomain.ErrTokenInvalid", err)

}

func TestCredentialVerifierChangePasswordMapsUpdateError(t *testing.T) {
	updateErr := errors.New("update failed")
	ctrl := gomock.NewController(t)
	repo := NewMockUserCredentialStore(ctrl)
	verifier := authcredentials.NewVerifier(repo, testPasswordService(t))
	repo.EXPECT().GetCredentialByUserID(gomock.Any(), authTestUserID).Return(&authdomain.UserCredential{UserID: authTestUserID, Status: identity.UserStatusMustChangePassword, TokenVersion: 2}, nil)
	repo.EXPECT().UpdateCredentials(gomock.Any(), gomock.Any()).Return(int64(0), updateErr)

	_, err := verifier.ChangePassword(context.Background(), authTestUserID, 2, "new-secret")
	require.ErrorIs(t, err, updateErr,
		"err = %v, want %v", err, updateErr)

}

func TestAuthTokenIssuerParsesBearerRefreshToken(t *testing.T) {
	cfg := &serviceconfig.Config{Auth: serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}}}
	jwt := commonauth.NewJWTService(commonauth.JWTConfig{Secret: cfg.Auth.JWT.Secret, Issuer: cfg.Auth.JWT.Issuer, Audience: cfg.Auth.JWT.Audience})
	issuer := authtokens.NewIssuer(jwt, cfg)
	pair, err := issuer.IssueTokenPair(context.Background(), authTestUserID.String(), 2, "s-123")
	require.NoError(t, err,
		"IssueTokenPair: %v", err)

	claims, err := issuer.ParseRefreshToken(context.Background(), "Bearer "+pair.Response.RefreshToken)
	require.NoError(t, err,
		"ParseRefreshToken: %v", err)
	require.False(t, claims.UserID != authTestUserID.String() || claims.SessionID != "s-123" || claims.Subject != authtokens.SubjectRefresh,
		"claims = %#v", claims)

}

func TestAuthSessionLifecycleRejectsRefreshVersionMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenVersions := NewMockTokenVersionCache(ctrl)
	sessions := NewMockRefreshSessionStore(ctrl)
	passwordChanges := NewMockPasswordChangeSessionStore(ctrl)
	lifecycle, err := authsessions.NewLifecycle(users, tokenVersions, sessions, passwordChanges, 5, noopTokenVersionInvalidator{})
	require.NoError(t, err)
	claims := &authtokens.Claims{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123"}

	sessions.EXPECT().GetSession(gomock.Any(), authTestUserID.String(), "s-123").Return(authRefreshTestSession("s-123", 2), nil)
	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(3), nil)

	_, _, err = lifecycle.ValidateRefreshSession(context.Background(), claims)
	require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
		"err = %v, want authdomain.ErrTokenInvalid", err)

}

func TestAuthSessionLifecycleRotateTokenSessionMapsRejectedSession(t *testing.T) {
	for _, err := range []error{authdomain.ErrAuthSessionNotFound, authdomain.ErrAuthSessionMismatch} {
		t.Run(err.Error(), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			lifecycle, _, _, sessions := newGeneratedAuthSessionLifecycle(t, ctrl)
			oldSession := authRefreshTestSession("s-old", 2)
			newSession := authRefreshTestSession("s-new", 2)
			sessions.EXPECT().RotateSession(gomock.Any(), oldSession, newSession, time.Hour, 5).Return(err)

			err := lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)
			require.ErrorIs(t, err, authdomain.ErrTokenInvalid,
				"err = %v, want authdomain.ErrTokenInvalid", err)

		})
	}
}

func TestAuthSessionLifecycleRotateTokenSessionMapsUnexpectedError(t *testing.T) {
	rotateErr := errors.New("redis failed")
	ctrl := gomock.NewController(t)
	lifecycle, _, _, sessions := newGeneratedAuthSessionLifecycle(t, ctrl)
	oldSession := authRefreshTestSession("s-old", 2)
	newSession := authRefreshTestSession("s-new", 2)
	sessions.EXPECT().RotateSession(gomock.Any(), oldSession, newSession, time.Hour, 5).Return(rotateErr)

	err := lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)
	require.ErrorIs(t, err, rotateErr,
		"err = %v, want %v", err, rotateErr)

}

func TestAuthSessionLifecycleCurrentTokenVersionUsesCacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, _, tokenVersions, _ := newGeneratedAuthSessionLifecycle(t, ctrl)
	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(2), nil)

	version, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())
	require.NoError(t, err,
		"currentTokenVersion: %v", err)
	require.EqualValues(t, 2, version,
		"version = %d, want 2", version)

}

func TestAuthSessionLifecycleCurrentTokenVersionCacheMissReadsRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, users, tokenVersions, _ := newGeneratedAuthSessionLifecycle(t, ctrl)

	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss)
	users.EXPECT().GetTokenVersion(gomock.Any(), authTestUserID).Return(int64(7), nil)
	tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), authTestUserID.String(), int64(7)).Return(nil)

	version, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())
	require.NoError(t, err,
		"currentTokenVersion: %v", err)
	require.EqualValues(t, 7, version,
		"version = %d, want 7", version)

}

func TestAuthSessionLifecycleCurrentTokenVersionCacheErrorReturnsInfrastructureError(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, _, tokenVersions, _ := newGeneratedAuthSessionLifecycle(t, ctrl)
	cacheErr := errors.New("redis failed")
	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(0), cacheErr)

	_, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())
	require.ErrorIs(t, err, cacheErr,
		"err = %v, want cache error", err)

}

func TestAuthSessionLifecycleCurrentTokenVersionDatabaseFallbackErrorReturnsInfrastructureError(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, users, tokenVersions, _ := newGeneratedAuthSessionLifecycle(t, ctrl)
	dbErr := errors.New("database failed")

	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss)
	users.EXPECT().GetTokenVersion(gomock.Any(), authTestUserID).Return(int64(0), dbErr)

	_, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())
	require.ErrorIs(t, err, dbErr,
		"err = %v, want database error", err)

}

func TestAuthSessionLifecycleCurrentTokenVersionBackfillErrorReturnsInfrastructureError(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, users, tokenVersions, _ := newGeneratedAuthSessionLifecycle(t, ctrl)
	cacheErr := errors.New("redis set failed")

	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss)
	users.EXPECT().GetTokenVersion(gomock.Any(), authTestUserID).Return(int64(7), nil)
	tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), authTestUserID.String(), int64(7)).Return(cacheErr)

	_, err := lifecycle.CurrentTokenVersion(context.Background(), authTestUserID.String())
	require.ErrorIs(t, err, cacheErr,
		"err = %v, want cache backfill error", err)

}

func TestAuthSessionLifecycleRevokeAllUserSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, users, tokenVersions, sessions := newGeneratedAuthSessionLifecycle(t, ctrl)

	users.EXPECT().IncrementTokenVersion(gomock.Any(), authTestUserID).Return(int64(4), nil)
	tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), authTestUserID.String(), int64(4)).Return(nil)
	sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), authTestUserID.String()).Return(nil)

	result, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)
	require.NoError(t, err,
		"RevokeAllUserSessions: %v", err)
	require.False(t, result.UserID != authTestUserID || result.TokenVersion != 4,
		"result = %#v", result)
	require.NoError(t, result.ProjectionError,
		"projection error = %v, want nil", result.ProjectionError)

}

func TestAuthSessionLifecycleRevokeAllUserSessionsMapsUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, users, _, _ := newGeneratedAuthSessionLifecycle(t, ctrl)
	users.EXPECT().IncrementTokenVersion(gomock.Any(), authTestUserID).Return(int64(0), identity.ErrUserNotFound)

	_, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)
	require.ErrorIs(t, err, identity.ErrUserNotFound,
		"err = %v, want ErrUserNotFound", err)

}

func TestAuthSessionLifecycleRevokeAllUserSessionsCompensatesCacheRefreshError(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, users, tokenVersions, sessions := newGeneratedAuthSessionLifecycle(t, ctrl)
	cacheErr := errors.New("cache refresh failed")

	users.EXPECT().IncrementTokenVersion(gomock.Any(), authTestUserID).Return(int64(4), nil)
	tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), authTestUserID.String(), int64(4)).Return(cacheErr)
	tokenVersions.EXPECT().DeleteCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(nil)
	sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), authTestUserID.String()).Return(nil)

	result, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)
	require.NoError(t, err,
		"RevokeAllUserSessions: %v", err)
	require.False(t, result.UserID != authTestUserID || result.TokenVersion != 4,
		"result = %#v", result)
	require.ErrorIs(t, result.ProjectionError, cacheErr,
		"projection error = %v, want cache error", result.ProjectionError)

}

func TestAuthSessionLifecycleRevokeAllUserSessionsSucceedsAfterDeleteAllProjectionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, users, tokenVersions, sessions := newGeneratedAuthSessionLifecycle(t, ctrl)
	deleteErr := errors.New("delete all failed")

	users.EXPECT().IncrementTokenVersion(gomock.Any(), authTestUserID).Return(int64(4), nil)
	tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), authTestUserID.String(), int64(4)).Return(nil)
	sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), authTestUserID.String()).Return(deleteErr)

	result, err := lifecycle.RevokeAllUserSessions(context.Background(), authTestUserID)
	require.NoError(t, err,
		"RevokeAllUserSessions: %v", err)
	require.False(t, result.UserID != authTestUserID || result.TokenVersion != 4,
		"result = %#v", result)
	require.ErrorIs(t, result.ProjectionError, deleteErr,
		"projection error = %v, want delete error", result.ProjectionError)

}

func TestAuthSessionLifecycleRevokeUserSessionsAtVersionReturnsProjectionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	lifecycle, _, tokenVersions, sessions := newGeneratedAuthSessionLifecycle(t, ctrl)
	cacheErr := errors.New("cache refresh failed")
	deleteErr := errors.New("delete all failed")
	deleteCacheErr := errors.New("delete cache failed")

	tokenVersions.EXPECT().CacheTokenVersion(gomock.Any(), authTestUserID.String(), int64(4)).Return(cacheErr)
	tokenVersions.EXPECT().DeleteCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(deleteCacheErr)
	sessions.EXPECT().DeleteAllUserSessions(gomock.Any(), authTestUserID.String()).Return(deleteErr)

	err := lifecycle.RevokeUserSessionsAtVersion(context.Background(), authTestUserID, 4)
	require.False(t, !errors.Is(err, cacheErr) || !errors.Is(err, deleteCacheErr) || !errors.Is(err, deleteErr),
		"err = %v, want cache, cache delete, and session delete errors", err)

}

func TestTokenVersionValidatorRejectsStaleTokenWhenCacheHasNewVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenVersions := NewMockTokenVersionCache(ctrl)
	validator := newTestTokenVersionValidator(t, users, tokenVersions)
	tokenVersions.EXPECT().GetCachedTokenVersion(gomock.Any(), authTestUserID.String()).Return(int64(4), nil)

	err := validator.ValidateTokenVersion(context.Background(), authTestUserID.String(), 3)
	require.ErrorIs(t, err, commonauth.ErrTokenVersionMismatch,
		"err = %v, want common token version mismatch", err)

	var mismatch *commonauth.TokenVersionMismatchError
	require.ErrorAs(t, err, &mismatch,
		"err = %v, want structured token version mismatch", err)
	require.False(t, mismatch.Current != 4 || mismatch.Token != 3,
		"mismatch = %#v, want current=4 token=3", mismatch)

}

func newTestTokenVersionValidator(t *testing.T, users authapplication.UserTokenVersionStore, tokenCache authapplication.TokenVersionCache) commonauth.TokenVersionValidator {
	t.Helper()
	cache, err := localcache.NewLoadingCache[string, int64](localcache.Config{
		Name:        "auth_token_version_test",
		Capacity:    100,
		TTL:         time.Minute,
		LoadTimeout: time.Second,
	}, func(ctx context.Context, userID string) (int64, error) {
		return authvalidators.Current(ctx, users, tokenCache, userID)
	})
	require.NoError(t, err,
		"New localcache: %v", err)

	t.Cleanup(cache.Close)
	return authvalidators.NewCachingValidator(cache)
}

func authRefreshTestSession(sessionID string, tokenVersion int64) authdomain.AuthSession {
	return authdomain.AuthSession{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}

func newGeneratedAuthSessionLifecycle(t testing.TB, ctrl *gomock.Controller) (authsessions.Lifecycle, *MockUserTokenVersionStore, *MockTokenVersionCache, *MockRefreshSessionStore) {
	t.Helper()
	users := NewMockUserTokenVersionStore(ctrl)
	tokenVersions := NewMockTokenVersionCache(ctrl)
	sessions := NewMockRefreshSessionStore(ctrl)
	passwordChanges := NewMockPasswordChangeSessionStore(ctrl)
	lifecycle, err := authsessions.NewLifecycle(users, tokenVersions, sessions, passwordChanges, 5, noopTokenVersionInvalidator{})
	require.NoError(t, err)
	return lifecycle, users, tokenVersions, sessions
}
