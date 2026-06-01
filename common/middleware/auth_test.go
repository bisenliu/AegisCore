package middleware

import (
	"encoding/json"
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
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.AuthConfig{JWT: config.JWTConfig{Secret: "secret"}, Whitelist: []string{"/healthz"}}
	validToken := signAuthTestToken(t, "secret", "u-123", time.Now().Add(time.Hour))

	tests := []struct {
		name          string
		path          string
		authorization string
		wantStatus    int
		wantHandled   bool
	}{
		{name: "whitelist", path: "/healthz", wantStatus: http.StatusOK, wantHandled: true},
		{name: "missing header", path: "/api/v1/users/123", wantStatus: http.StatusUnauthorized},
		{name: "invalid format", path: "/api/v1/users/123", authorization: "Token abc", wantStatus: http.StatusUnauthorized},
		{name: "empty token", path: "/api/v1/users/123", authorization: "Bearer ", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", path: "/api/v1/users/123", authorization: "Bearer invalid", wantStatus: http.StatusUnauthorized},
		{name: "valid token", path: "/api/v1/users/123", authorization: "Bearer " + validToken, wantStatus: http.StatusOK, wantHandled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(Auth(commonjwt.NewService(cfg), cfg))
			handled := false
			engine.GET("/*path", func(c *gin.Context) {
				handled = true
				if tt.authorization != "" && tt.wantStatus == http.StatusOK {
					if got, ok := c.Get(contextutil.UserIDKey); !ok || got != "u-123" {
						t.Fatalf("gin user id = %#v, %v; want u-123, true", got, ok)
					}
					if got, ok := contextutil.UserIDFromContext(c.Request.Context()); !ok || got != "u-123" {
						t.Fatalf("context user id = %q, %v; want u-123, true", got, ok)
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
				if envelope.Success || envelope.Code != response.CodeUnauthenticated {
					t.Fatalf("envelope = %#v", envelope)
				}
			}
		})
	}
}

func signAuthTestToken(t *testing.T, secret, userID string, expiresAt time.Time) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, commonjwt.Claims{
		UserID:           userID,
		RegisteredClaims: jwtv5.RegisteredClaims{ExpiresAt: jwtv5.NewNumericDate(expiresAt)},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return token
}
