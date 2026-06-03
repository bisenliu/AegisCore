package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/contextutil"
	commonjwt "github.com/aegiscore/common/jwt"
	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap/zaptest"
)

const authTestUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}, Whitelist: []string{"/healthz"}}
	validToken := signAuthTestToken(t, "secret", authTestUserID, 1, "s-123", time.Now().Add(time.Hour))
	expiredToken := signAuthTestToken(t, "secret", authTestUserID, 1, "s-123", time.Now().Add(-time.Hour))
	missingVersionToken := signAuthTestToken(t, "secret", authTestUserID, 0, "s-123", time.Now().Add(time.Hour))
	missingSessionToken := signAuthTestToken(t, "secret", authTestUserID, 1, "", time.Now().Add(time.Hour))
	passwordChangeToken := signAuthSubjectTestToken(t, "secret", commonjwt.SubjectPasswordChange, authTestUserID, 1, "pc-123", time.Now().Add(time.Hour))

	tests := []struct {
		name          string
		path          string
		authorization string
		wantStatus    int
		wantCode      response.Code
		wantHandled   bool
		validator     TokenVersionValidator
	}{
		{name: "whitelist", path: "/healthz", wantStatus: http.StatusOK, wantHandled: true},
		{name: "missing header", path: "/api/v1/users/123", wantStatus: http.StatusUnauthorized, wantCode: response.CodeUnauthenticated},
		{name: "invalid format", path: "/api/v1/users/123", authorization: "Token abc", wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenInvalid},
		{name: "empty token", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix, wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenInvalid},
		{name: "invalid token", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + "invalid", wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenInvalid},
		{name: "expired token", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + expiredToken, wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenExpired},
		{name: "missing token version", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + missingVersionToken, wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenInvalid},
		{name: "missing session id", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + missingSessionToken, wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenInvalid},
		{name: "password change token rejected", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + passwordChangeToken, wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenInvalid},
		{name: "token version mismatch", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + validToken, wantStatus: http.StatusUnauthorized, wantCode: response.CodeTokenInvalid, validator: TokenVersionValidatorFunc(func(context.Context, string, int64) error { return errors.New("version mismatch") })},
		{name: "valid token", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + validToken, wantStatus: http.StatusOK, wantHandled: true},
		{name: "valid token with version validator", path: "/api/v1/users/123", authorization: contextutil.TokenPrefix + validToken, wantStatus: http.StatusOK, wantHandled: true, validator: TokenVersionValidatorFunc(func(_ context.Context, userID string, version int64) error {
			if userID != authTestUserID || version != 1 {
				return errors.New("unexpected token version input")
			}
			return nil
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(AuthWithTokenVersionValidator(zaptest.NewLogger(t), commonjwt.NewService(cfg), cfg, tt.validator))
			handled := false
			engine.GET("/*path", func(c *gin.Context) {
				handled = true
				if tt.authorization != "" && tt.wantStatus == http.StatusOK {
					if got, ok := c.Get(contextutil.UserIDKey); !ok || got != authTestUserID {
						t.Fatalf("gin user id = %#v, %v; want %s, true", got, ok, authTestUserID)
					}
					if got, ok := contextutil.UserIDFromContext(c.Request.Context()); !ok || got != authTestUserID {
						t.Fatalf("context user id = %q, %v; want %s, true", got, ok, authTestUserID)
					}
					if got, ok := c.Get(contextutil.SessionIDKey); !ok || got != "s-123" {
						t.Fatalf("gin session id = %#v, %v; want s-123, true", got, ok)
					}
					if got, ok := contextutil.SessionIDFromContext(c.Request.Context()); !ok || got != "s-123" {
						t.Fatalf("context session id = %q, %v; want s-123, true", got, ok)
					}
				}
				c.Status(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.authorization != "" {
				request.Header.Set(contextutil.AuthorizationHeader, tt.authorization)
			}
			engine.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if handled != tt.wantHandled {
				t.Fatalf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if tt.wantStatus == http.StatusUnauthorized {
				var envelope response.Envelope
				if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if envelope.Success || envelope.Code != tt.wantCode || envelope.Message != response.MessageAuthInvalid {
					t.Fatalf("envelope = %#v", envelope)
				}
			}
		})
	}
}

func signAuthTestToken(t *testing.T, secret, userID string, tokenVersion int64, sessionID string, expiresAt time.Time) string {
	return signAuthSubjectTestToken(t, secret, commonjwt.SubjectAccess, userID, tokenVersion, sessionID, expiresAt)
}

func signAuthSubjectTestToken(t *testing.T, secret, subject, userID string, tokenVersion int64, sessionID string, expiresAt time.Time) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, commonjwt.Claims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        sessionID,
		RegisteredClaims: jwtv5.RegisteredClaims{Subject: subject, ExpiresAt: jwtv5.NewNumericDate(expiresAt)},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
