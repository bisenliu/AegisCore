package auth

import (
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
)

const testUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
const testTokenID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d5e"

func TestJWTServiceParseToken(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}})

	t.Run("valid token", func(t *testing.T) {
		claims, err := service.ParseToken(signTestToken(t, "secret", Claims{
			UserID:       testUserID,
			TokenVersion: 1,
			SessionID:    "s-123",
			RegisteredClaims: jwtv5.RegisteredClaims{
				ID:        testTokenID,
				Issuer:    "issuer",
				Audience:  []string{"audience"},
				Subject:   SubjectAccess,
				ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}))
		require.NoError(t, err)
		require.Equal(t, testUserID, claims.UserID)
		require.Equal(t, int64(1), claims.TokenVersion)
		require.Equal(t, "s-123", claims.SessionID)
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
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(-time.Hour))}},
		},
		{
			name:   "wrong secret",
			secret: "other-secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "issuer mismatch",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "other", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "audience mismatch",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "issuer", Audience: []string{"other"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
		},
		{
			name:   "missing token id",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingTokenID,
		},
		{
			name:   "invalid token id format",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: "not-a-uuid", Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrInvalidTokenID,
		},
		{
			name:   "missing user id",
			secret: "secret",
			claims: Claims{TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingUserID,
		},
		{
			name:   "invalid user id",
			secret: "secret",
			claims: Claims{UserID: "not-a-uuid", TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrInvalidUserID,
		},
		{
			name:   "missing token version",
			secret: "secret",
			claims: Claims{UserID: testUserID, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingTokenVersion,
		},
		{
			name:   "missing session id",
			secret: "secret",
			claims: Claims{UserID: testUserID, TokenVersion: 1, RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Issuer: "issuer", Audience: []string{"audience"}, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}},
			want:   ErrMissingSessionID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ParseToken(signTestToken(t, tt.secret, tt.claims))
			require.Error(t, err)
			if tt.want != nil {
				require.ErrorIs(t, err, tt.want)
			}
			if tt.want == ErrInvalidUserID {
				require.NotErrorIs(t, err, ErrMissingUserID)
			}
		})
	}
}

func TestJWTServiceParseTokenOptionalIssuerAudience(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	token := signTestToken(t, "secret", Claims{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", RegisteredClaims: jwtv5.RegisteredClaims{ID: testTokenID, Subject: SubjectAccess, ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour))}})
	_, err := service.ParseToken(token)
	require.NoError(t, err)
}

func TestJWTServiceSignTokens(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"}})
	token, err := service.SignAccessToken(SignInput{UserID: testUserID, TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	require.NoError(t, err)
	claims, err := service.ParseToken(token)
	require.NoError(t, err)
	require.Equal(t, testUserID, claims.UserID)
	require.Equal(t, int64(2), claims.TokenVersion)
	require.Equal(t, "s-123", claims.SessionID)
	require.Equal(t, SubjectAccess, claims.Subject)
	assertValidTokenID(t, claims.ID)

	refresh, err := service.SignRefreshToken(SignInput{UserID: testUserID, TokenVersion: 2, SessionID: "s-123", TTL: time.Hour})
	require.NoError(t, err)
	refreshClaims, err := service.ParseRefreshToken(refresh)
	require.NoError(t, err)
	require.Equal(t, SubjectRefresh, refreshClaims.Subject)
	assertValidTokenID(t, refreshClaims.ID)

	passwordChange, err := service.SignPasswordChangeToken(SignInput{UserID: testUserID, TokenVersion: 2, SessionID: "pc-123", TTL: time.Hour})
	require.NoError(t, err)
	passwordChangeClaims, err := service.ParsePasswordChangeToken(passwordChange)
	require.NoError(t, err)
	require.Equal(t, SubjectPasswordChange, passwordChangeClaims.Subject)
	require.Equal(t, "pc-123", passwordChangeClaims.SessionID)
	assertValidTokenID(t, passwordChangeClaims.ID)
}

func TestJWTServiceRejectsMissingTokenID(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	tests := []struct {
		name    string
		subject string
		parse   func(string) (*Claims, error)
	}{
		{name: "access token", subject: SubjectAccess, parse: service.ParseToken},
		{name: "refresh token", subject: SubjectRefresh, parse: service.ParseRefreshToken},
		{name: "password change token", subject: SubjectPasswordChange, parse: service.ParsePasswordChangeToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := signTestToken(t, "secret", Claims{
				UserID:       testUserID,
				TokenVersion: 1,
				SessionID:    "s-123",
				RegisteredClaims: jwtv5.RegisteredClaims{
					Subject:   tt.subject,
					ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
				},
			})
			_, err := tt.parse(token)
			require.ErrorIs(t, err, ErrMissingTokenID)
		})
	}
}

func TestJWTServiceSignTokensInvalidUserID(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	input := SignInput{UserID: "not-a-uuid", TokenVersion: 1, SessionID: "s-123", TTL: time.Hour}

	_, err := service.SignAccessToken(input)
	require.ErrorIs(t, err, ErrInvalidUserID)
	_, err = service.SignRefreshToken(input)
	require.ErrorIs(t, err, ErrInvalidUserID)
	_, err = service.SignPasswordChangeToken(input)
	require.ErrorIs(t, err, ErrInvalidUserID)
}

func TestJWTServiceRejectsWrongSubjects(t *testing.T) {
	service := NewJWTService(config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}})
	refresh, err := service.SignRefreshToken(SignInput{UserID: testUserID, TokenVersion: 1, SessionID: "s-123", TTL: time.Hour})
	require.NoError(t, err)
	_, err = service.ParseToken(refresh)
	require.ErrorIs(t, err, ErrInvalidSubject)
	passwordChange, err := service.SignPasswordChangeToken(SignInput{UserID: testUserID, TokenVersion: 1, SessionID: "pc-123", TTL: time.Hour})
	require.NoError(t, err)
	_, err = service.ParseToken(passwordChange)
	require.ErrorIs(t, err, ErrInvalidSubject)
}

func TestJWTServiceParseTokenMissingSecret(t *testing.T) {
	service := NewJWTService(config.AuthConfig{})
	_, err := service.ParseToken("token")
	require.ErrorIs(t, err, ErrMissingSecret)
}

func signTestToken(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}

func assertValidTokenID(t *testing.T, tokenID string) {
	t.Helper()
	parsed, err := uuid.Parse(tokenID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())
}
