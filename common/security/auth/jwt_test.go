package auth

import (
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type testClaims struct {
	UserID string `json:"user_id"`
	jwtv5.RegisteredClaims
}

const testUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

func TestJWTServiceVerifyClaims(t *testing.T) {
	service := NewJWTService(JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"})
	token := signTestToken(t, "secret", testClaims{UserID: testUserID, RegisteredClaims: jwtv5.RegisteredClaims{ID: "jti", Issuer: "issuer", Audience: []string{"audience"}, Subject: "access", ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}})
	var claims testClaims
	require.NoError(t, service.VerifyClaims(token, &claims))
	require.Equal(t, testUserID, claims.UserID)
}

func TestJWTServiceVerifyClaimsRejectsInvalidInput(t *testing.T) {
	service := NewJWTService(JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"})
	tests := []struct {
		name   string
		secret string
		claims testClaims
	}{
		{name: "expired token", secret: "secret", claims: testClaims{RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(-time.Hour))}}},
		{name: "wrong secret", secret: "other", claims: testClaims{RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}}},
		{name: "issuer mismatch", secret: "secret", claims: testClaims{RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "other", Audience: []string{"audience"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}}},
		{name: "audience mismatch", secret: "secret", claims: testClaims{RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"other"}, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var claims testClaims
			require.Error(t, service.VerifyClaims(signTestToken(t, tt.secret, tt.claims), &claims))
		})
	}
}

func TestJWTServiceVerifyClaimsOptionalIssuerAudience(t *testing.T) {
	service := NewJWTService(JWTConfig{Secret: "secret"})
	token := signTestToken(t, "secret", testClaims{UserID: testUserID, RegisteredClaims: jwtv5.RegisteredClaims{ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}})
	var claims testClaims
	require.NoError(t, service.VerifyClaims(token, &claims))
}

func TestJWTServiceVerifyClaimsMissingSecret(t *testing.T) {
	service := NewJWTService(JWTConfig{})
	var claims testClaims
	require.ErrorIs(t, service.VerifyClaims("token", &claims), ErrMissingSecret)
}

func signTestToken(t *testing.T, secret string, claims testClaims) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}
