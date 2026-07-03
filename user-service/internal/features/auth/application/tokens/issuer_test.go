package tokens

import (
	"context"
	"testing"
	"time"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
)

const issuerTestUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

func TestIssuerUsesDefaultTTLs(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}}}
	issuer := NewIssuer(commonauth.NewJWTService(cfg.Auth), cfg)

	tokens, err := issuer.IssueTokenPair(context.Background(), issuerTestUserID, 2, "s-123")

	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}
	if tokens.Response.ExpiresIn != int64(defaultAccessTokenTTL.Seconds()) {
		t.Fatalf("ExpiresIn = %d, want %d", tokens.Response.ExpiresIn, int64(defaultAccessTokenTTL.Seconds()))
	}
	if tokens.RefreshTTL != defaultRefreshTokenTTL {
		t.Fatalf("RefreshTTL = %s, want %s", tokens.RefreshTTL, defaultRefreshTokenTTL)
	}
}

func TestIssuerIssuesPasswordChangeToken(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}}}
	jwt := commonauth.NewJWTService(cfg.Auth)
	issuer := NewIssuer(jwt, cfg)

	tokens, err := issuer.IssuePasswordChangeToken(context.Background(), issuerTestUserID, 2, "pc-123")
	if err != nil {
		t.Fatalf("IssuePasswordChangeToken: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken != "" || tokens.TokenType != commonauth.TokenTypeBearer || tokens.ExpiresIn != int64((15*time.Minute).Seconds()) || !tokens.PasswordChangeRequired {
		t.Fatalf("tokens = %#v", tokens)
	}

	claims, parsedUserID, err := issuer.ParsePasswordChangeToken(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatalf("ParsePasswordChangeToken: %v", err)
	}
	if parsedUserID.String() != issuerTestUserID || claims.Subject != commonauth.SubjectPasswordChange || claims.SessionID != "pc-123" || claims.TokenVersion != 2 {
		t.Fatalf("claims = %#v parsedUserID = %s", claims, parsedUserID.String())
	}
}

func TestIssuerParsesBearerRefreshToken(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}}}
	jwt := commonauth.NewJWTService(cfg.Auth)
	issuer := NewIssuer(jwt, cfg)
	refresh, err := jwt.SignRefreshToken(commonauth.SignInput{UserID: issuerTestUserID, TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}

	claims, err := issuer.ParseRefreshToken(context.Background(), "Bearer "+refresh)

	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if claims.UserID != issuerTestUserID || claims.SessionID != "s-123" || claims.Subject != commonauth.SubjectRefresh {
		t.Fatalf("claims = %#v", claims)
	}
}
