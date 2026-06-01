package jwt

import (
	"errors"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func TestServiceParseToken(t *testing.T) {
	service := NewService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}})

	t.Run("valid token", func(t *testing.T) {
		claims, err := service.ParseToken(signTestToken(t, "secret", Claims{
			UserID: "u-123",
			RegisteredClaims: jwtv5.RegisteredClaims{
				Issuer:    "issuer",
				Audience:  []string{"audience"},
				ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}))
		if err != nil {
			t.Fatalf("ParseToken: %v", err)
		}
		if claims.UserID != "u-123" {
			t.Fatalf("UserID = %q, want u-123", claims.UserID)
		}
	})

	tests := []struct {
		name   string
		secret string
		claims Claims
		want   error
	}{
		{
			name:   "expired token",
			secret: "secret",
			claims: Claims{UserID: "u-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(-time.Hour))}},
		},
		{
			name:   "wrong secret",
			secret: "other-secret",
			claims: Claims{UserID: "u-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "issuer mismatch",
			secret: "secret",
			claims: Claims{UserID: "u-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "other", Audience: []string{"audience"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "audience mismatch",
			secret: "secret",
			claims: Claims{UserID: "u-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"other"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "missing user id",
			secret: "secret",
			claims: Claims{RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingUserID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ParseToken(signTestToken(t, tt.secret, tt.claims))
			if err == nil {
				t.Fatal("ParseToken err = nil, want error")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("ParseToken err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestServiceParseTokenOptionalIssuerAudience(t *testing.T) {
	service := NewService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	token := signTestToken(t, "secret", Claims{UserID: "u-123", RegisteredClaims: jwtv5.RegisteredClaims{ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}})
	if _, err := service.ParseToken(token); err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
}

func TestServiceParseTokenMissingSecret(t *testing.T) {
	service := NewService(config.AuthConfig{})
	_, err := service.ParseToken("token")
	if !errors.Is(err, ErrMissingSecret) {
		t.Fatalf("ParseToken err = %v, want %v", err, ErrMissingSecret)
	}
}

func signTestToken(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
