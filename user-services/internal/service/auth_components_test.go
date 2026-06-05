package service

import (
	"context"
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
	manager := newAuthSessionManager(&sessionStoreStub{version: 3, session: authRefreshTestSession("s-123", 2)})
	claims := &auth.Claims{UserID: authTestUserID.String(), TokenVersion: 2, SessionID: "s-123"}

	_, _, err := manager.ValidateRefreshSession(context.Background(), claims)

	appErr := response.FromError(err)
	if appErr.Code != response.CodeTokenInvalid {
		t.Fatalf("err = %#v", appErr)
	}
}

func authRefreshTestSession(sessionID string, tokenVersion int64) repository.AuthSession {
	return repository.AuthSession{UserID: authTestUserID.String(), SessionID: sessionID, TokenVersion: tokenVersion}
}
