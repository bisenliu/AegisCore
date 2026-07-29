package tokens

import (
	"context"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	commonauth "github.com/aegiscore/common/security/auth"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

var issuerTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestIssuerUsesDefaultTTLs(t *testing.T) {
	cfg := testIssuerConfig(serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience"})
	issuer := NewIssuer(testJWTVerifier(cfg), cfg)

	tokens, err := issuer.IssueTokenPair(context.Background(), issuerTestUserID, 2, "s-123")
	require.NoError(t, err,
		"IssueTokenPair: %v", err)
	require.Equal(t, int64(defaultAccessTokenTTL.Seconds()), tokens.Response.ExpiresIn,
		"ExpiresIn = %d, want %d", tokens.Response.ExpiresIn, int64(defaultAccessTokenTTL.Seconds()))
	require.Equal(t, defaultRefreshTokenTTL, tokens.RefreshTTL,
		"RefreshTTL = %s, want %s", tokens.RefreshTTL, defaultRefreshTokenTTL)

}

func TestIssuerIssuesPasswordChangeToken(t *testing.T) {
	cfg := testIssuerConfig(serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour, PasswordChangeTokenTTL: 4 * time.Minute})
	issuer := NewIssuer(testJWTVerifier(cfg), cfg)

	tokens, err := issuer.IssuePasswordChangeToken(context.Background(), issuerTestUserID, 2, "pc-123")
	require.NoError(t, err,
		"IssuePasswordChangeToken: %v", err)
	require.False(t, tokens.AccessToken == "" || tokens.RefreshToken != "" || tokens.TokenType != commonauth.TokenTypeBearer || tokens.ExpiresIn != int64((4*time.Minute).Seconds()),
		"tokens = %#v", tokens)

	claims, parsedUserID, err := issuer.ParsePasswordChangeToken(context.Background(), tokens.AccessToken)
	require.NoError(t, err,
		"ParsePasswordChangeToken: %v", err)
	require.False(t, parsedUserID != issuerTestUserID || claims.Subject != SubjectPasswordChange || claims.SessionID != "pc-123" || claims.TokenVersion != 2 || claims.ID == "",
		"claims = %#v parsedUserID = %s", claims, parsedUserID.String())

}

func TestIssuerUsesDefaultPasswordChangeTokenTTL(t *testing.T) {
	cfg := testIssuerConfig(serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour})
	issuer := NewIssuer(testJWTVerifier(cfg), cfg)

	tokens, err := issuer.IssuePasswordChangeToken(context.Background(), issuerTestUserID, 2, "pc-123")
	require.NoError(t, err,
		"IssuePasswordChangeToken: %v", err)
	require.Equal(t, int64(defaultPasswordChangeTokenTTL.Seconds()), tokens.ExpiresIn,
		"ExpiresIn = %d, want %d", tokens.ExpiresIn, int64(defaultPasswordChangeTokenTTL.Seconds()))
}

func TestIssuerParsesBearerRefreshToken(t *testing.T) {
	cfg := testIssuerConfig(serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour})
	issuer := NewIssuer(testJWTVerifier(cfg), cfg)
	pair, err := issuer.IssueTokenPair(context.Background(), issuerTestUserID, 2, "s-123")
	require.NoError(t, err,
		"SignRefreshToken: %v", err)

	claims, parsedUserID, err := issuer.ParseRefreshToken(context.Background(), "Bearer "+pair.Response.RefreshToken)
	require.NoError(t, err,
		"ParseRefreshToken: %v", err)
	require.False(t, parsedUserID != issuerTestUserID || claims.UserID != issuerTestUserID || claims.SessionID != "s-123" || claims.Subject != SubjectRefresh,
		"claims = %#v", claims)

}

func TestIssuerRejectsWrongSubjectForAccessVerifier(t *testing.T) {
	cfg := testIssuerConfig(serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour})
	issuer := NewAccessTokenVerifier(testJWTVerifier(cfg), cfg)
	passwordToken, err := NewIssuer(testJWTVerifier(cfg), cfg).IssuePasswordChangeToken(context.Background(), issuerTestUserID, 2, "pc-123")
	require.NoError(t, err)

	_, err = issuer.VerifyAccessToken(passwordToken.AccessToken)
	require.ErrorIs(t, err, errInvalidSubject)
}

func TestIssuerRejectsRefreshTokenMissingJTI(t *testing.T) {
	cfg := testIssuerConfig(serviceconfig.JWTConfig{Secret: "secret", Issuer: "issuer", Audience: "audience", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour})
	issuer := NewIssuer(testJWTVerifier(cfg), cfg)
	token := signIssuerTestClaims(t, cfg.JWT.Secret, Claims{
		UserID:       issuerTestUserID,
		TokenVersion: 2,
		SessionID:    "s-123",
		RegisteredClaims: jwtv5.RegisteredClaims{
			Issuer:    cfg.JWT.Issuer,
			Audience:  jwtv5.ClaimStrings{cfg.JWT.Audience},
			Subject:   SubjectRefresh,
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	_, _, err := issuer.ParseRefreshToken(context.Background(), token)
	require.Error(t, err)
}

func testIssuerConfig(jwt serviceconfig.JWTConfig) serviceconfig.AuthSettings {
	return serviceconfig.AuthSettings{JWT: jwt}
}

func testJWTVerifier(settings serviceconfig.AuthSettings) *commonauth.JWTService {
	return commonauth.NewJWTService(commonauth.JWTConfig{Secret: settings.JWT.Secret, Issuer: settings.JWT.Issuer, Audience: settings.JWT.Audience})
}

func signIssuerTestClaims(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}
