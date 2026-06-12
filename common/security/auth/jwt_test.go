package auth

import (
	"errors"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	"github.com/aegiscore/common/runtime/config"
)

const testUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

func TestJWTServiceParseToken(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}})

	t.Run("valid token", func(t *testing.T) {
		claims, err := service.ParseToken(signTestToken(t, "secret", Claims{
			UserID:       testUserID,
			TokenVersion: 1,
			SessionID:    "s-123",
			RegisteredClaims: jwtv5.RegisteredClaims{
				Issuer:    "issuer",
				Audience:  []string{"audience"},
				Subject:   SubjectAccess,
				ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}))
		if err != nil {
			t.Fatalf("ParseToken: %v", err)
		}
		if claims.UserID != testUserID {
			t.Fatalf("UserID = %q, want %s", claims.UserID, testUserID)
		}
		if claims.TokenVersion != 1 || claims.SessionID != "s-123" {
			t.Fatalf("claims = %#v, want token version and session", claims)
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
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(-time.Hour))}},
		},
		{
			name:   "wrong secret",
			secret: "other-secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "issuer mismatch",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "other", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "audience mismatch",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"other"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "missing user id",
			secret: "secret",
			claims: Claims{TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingUserID,
		},
		{
			name:   "invalid user id",
			secret: "secret",
			claims: Claims{UserID: "not-a-uuid", TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrInvalidUserID,
		},
		{
			name:   "missing token version",
			secret: "secret",
			claims: Claims{UserID: testUserID, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingTokenVersion,
		},
		{
			name:   "missing session id",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingSessionID,
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
			if tt.want == ErrInvalidUserID && errors.Is(err, ErrMissingUserID) {
				t.Fatalf("ParseToken err = %v, must not match %v", err, ErrMissingUserID)
			}
		})
	}
}

func TestJWTServiceParseTokenOptionalIssuerAudience(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	token := signTestToken(t, "secret", Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}})
	if _, err := service.ParseToken(token); err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
}

func TestJWTServiceSignTokens(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}})
	token, err := service.SignAccessToken(SignInput{UserID: testUserID, TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}
	claims, err := service.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != testUserID || claims.TokenVersion != 2 || claims.SessionID != "s-123" || claims.Subject != SubjectAccess {
		t.Fatalf("claims = %#v", claims)
	}

	refresh, err := service.SignRefreshToken(SignInput{UserID: testUserID, TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}
	refreshClaims, err := service.ParseRefreshToken(refresh)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if refreshClaims.Subject != SubjectRefresh {
		t.Fatalf("Subject = %q, want %s", refreshClaims.Subject, SubjectRefresh)
	}

	passwordChange, err := service.SignPasswordChangeToken(SignInput{UserID: testUserID, TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}
	passwordChangeClaims, err := service.ParsePasswordChangeToken(passwordChange)
	if err != nil {
		t.Fatalf("ParsePasswordChangeToken: %v", err)
	}
	if passwordChangeClaims.Subject != SubjectPasswordChange || passwordChangeClaims.SessionID != "pc-123" {
		t.Fatalf("passwordChangeClaims = %#v", passwordChangeClaims)
	}
}

func TestJWTServiceSignTokensInvalidUserID(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	input := SignInput{UserID: "not-a-uuid", TokenVersion: 1, SessionID: "s-123", TTL: time.Hour}

	if _, err := service.SignAccessToken(input); !errors.Is(err, ErrInvalidUserID) {
		t.Fatalf("SignAccessToken err = %v, want %v", err, ErrInvalidUserID)
	}
	if _, err := service.SignRefreshToken(input); !errors.Is(err, ErrInvalidUserID) {
		t.Fatalf("SignRefreshToken err = %v, want %v", err, ErrInvalidUserID)
	}
	if _, err := service.SignPasswordChangeToken(input); !errors.Is(err, ErrInvalidUserID) {
		t.Fatalf("SignPasswordChangeToken err = %v, want %v", err, ErrInvalidUserID)
	}
}

func TestJWTServiceRejectsWrongSubjects(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	refresh, err := service.SignRefreshToken(SignInput{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignRefreshToken: %v", err)
	}
	if _, err := service.ParseToken(refresh); !errors.Is(err, ErrInvalidSubject) {
		t.Fatalf("ParseToken err = %v, want %v", err, ErrInvalidSubject)
	}
	passwordChange, err := service.SignPasswordChangeToken(SignInput{UserID: testUserID, TokenVersion: 1, SessionID: "pc-123", TTL: time.Hour})
	if err != nil {
		t.Fatalf("SignPasswordChangeToken: %v", err)
	}
	if _, err := service.ParseToken(passwordChange); !errors.Is(err, ErrInvalidSubject) {
		t.Fatalf("ParseToken password-change err = %v, want %v", err, ErrInvalidSubject)
	}
}

func TestJWTServiceParseTokenMissingSecret(t *testing.T) {
	service := NewJWTService(config.AuthConfig{})
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
