package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/security/auth"
)

const authTestUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := testAccessTokenVerifier{secret: "secret"}
	validToken := signAuthTestToken(t, "secret", authTestUserID, 1, "s-123", time.Now().Add(time.Hour))
	expiredToken := signAuthTestToken(t, "secret", authTestUserID, 1, "s-123", time.Now().Add(-time.Hour))
	missingVersionToken := signAuthTestToken(t, "secret", authTestUserID, 0, "s-123", time.Now().Add(time.Hour))
	missingSessionToken := signAuthTestToken(t, "secret", authTestUserID, 1, "", time.Now().Add(time.Hour))
	passwordChangeToken := signAuthSubjectTestToken(t, "secret", "password_change", authTestUserID, 1, "pc-123", time.Now().Add(time.Hour))

	tests := []struct {
		name           string
		path           string
		verifier       auth.AccessTokenVerifier
		authorization  string
		wantStatus     int
		wantCode       contracterrors.Code
		wantHandled    bool
		setupValidator func(ctrl *gomock.Controller) auth.TokenVersionValidator
		wantLogLevel   zapcore.Level
		wantLogMsg     string
		wantMismatch   bool
	}{
		{name: "missing header", path: "/api/v1/users/123", wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeUnauthenticated, wantLogLevel: zapcore.InfoLevel, wantLogMsg: "missing authorization header"},
		{name: "invalid format", path: "/api/v1/users/123", authorization: "Token abc", wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "invalid authorization header format"},
		{name: "empty token", path: "/api/v1/users/123", authorization: auth.TokenPrefix, wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "empty bearer token"},
		{name: "invalid token", path: "/api/v1/users/123", authorization: auth.TokenPrefix + "invalid", wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "token validation failed"},
		{name: "expired token", path: "/api/v1/users/123", authorization: auth.TokenPrefix + expiredToken, wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenExpired, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "token validation failed"},
		{name: "missing token version", path: "/api/v1/users/123", authorization: auth.TokenPrefix + missingVersionToken, wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "token validation failed"},
		{name: "missing session id", path: "/api/v1/users/123", authorization: auth.TokenPrefix + missingSessionToken, wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "token validation failed"},
		{name: "password change token rejected", path: "/api/v1/users/123", authorization: auth.TokenPrefix + passwordChangeToken, wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "token validation failed"},
		{name: "token version mismatch", path: "/api/v1/users/123", authorization: auth.TokenPrefix + validToken, wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, setupValidator: func(ctrl *gomock.Controller) auth.TokenVersionValidator {
			validator := NewMockTokenVersionValidator(ctrl)
			validator.EXPECT().ValidateTokenVersion(gomock.Any(), authTestUserID, int64(1)).Return(fmt.Errorf("validate token version: %w", &auth.TokenVersionMismatchError{Current: 3, Token: 1}))
			return validator
		}, wantLogLevel: zapcore.WarnLevel, wantLogMsg: "token version mismatch", wantMismatch: true},
		{name: "token version infrastructure error", path: "/api/v1/users/123", authorization: auth.TokenPrefix + validToken, wantStatus: http.StatusInternalServerError, wantCode: contracterrors.CodeInternalError, setupValidator: func(ctrl *gomock.Controller) auth.TokenVersionValidator {
			validator := NewMockTokenVersionValidator(ctrl)
			validator.EXPECT().ValidateTokenVersion(gomock.Any(), authTestUserID, int64(1)).Return(errors.New("redis unavailable"))
			return validator
		}, wantLogLevel: zapcore.ErrorLevel, wantLogMsg: "token version validation failed"},
		{name: "missing jwt secret", path: "/api/v1/users/123", verifier: testAccessTokenVerifier{}, authorization: auth.TokenPrefix + validToken, wantStatus: http.StatusUnauthorized, wantCode: contracterrors.CodeTokenInvalid, wantLogLevel: zapcore.ErrorLevel, wantLogMsg: "token validation failed"},
		{name: "valid token", path: "/api/v1/users/123", authorization: auth.TokenPrefix + validToken, wantStatus: http.StatusOK, wantHandled: true},
		{name: "valid token with lowercase bearer prefix", path: "/api/v1/users/123", authorization: "bearer " + validToken, wantStatus: http.StatusOK, wantHandled: true},
		{name: "valid token with uppercase bearer prefix", path: "/api/v1/users/123", authorization: "BEARER " + validToken, wantStatus: http.StatusOK, wantHandled: true},
		{name: "valid token with version validator", path: "/api/v1/users/123", authorization: auth.TokenPrefix + validToken, wantStatus: http.StatusOK, wantHandled: true, setupValidator: func(ctrl *gomock.Controller) auth.TokenVersionValidator {
			validator := NewMockTokenVersionValidator(ctrl)
			validator.EXPECT().ValidateTokenVersion(gomock.Any(), authTestUserID, int64(1)).Return(nil)
			return validator
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			var validator auth.TokenVersionValidator
			if tt.setupValidator != nil {
				validator = tt.setupValidator(ctrl)
			}
			testVerifier := auth.AccessTokenVerifier(verifier)
			if tt.verifier != nil {
				testVerifier = tt.verifier
			}
			core, logs := observer.New(zapcore.DebugLevel)
			engine := gin.New()
			require.NoError(t, engine.SetTrustedProxies([]string{"10.0.0.1"}))
			engine.Use(AuthWithTokenVersionValidator(zap.New(core), testVerifier, validator))
			handled := false
			engine.GET("/*path", func(c *gin.Context) {
				handled = true
				if tt.authorization != "" && tt.wantStatus == http.StatusOK {
					if got, ok := c.Get(auth.UserIDKey); !ok || got != authTestUserID {
						require.True(t, ok)
						require.Equal(t, authTestUserID, got)
					}
					if got, ok := auth.UserIDFromContext(c.Request.Context()); !ok || got != authTestUserID {
						require.True(t, ok)
						require.Equal(t, authTestUserID, got)
					}
					if got, ok := c.Get(auth.SessionIDKey); !ok || got != "s-123" {
						require.True(t, ok)
						require.Equal(t, "s-123", got)
					}
					if got, ok := auth.SessionIDFromContext(c.Request.Context()); !ok || got != "s-123" {
						require.True(t, ok)
						require.Equal(t, "s-123", got)
					}
				}
				c.Status(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.RemoteAddr = "10.0.0.1:12345"
			request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
			request.Header.Set("User-Agent", "auth-test-agent")
			if tt.authorization != "" {
				request.Header.Set(auth.AuthorizationHeader, tt.authorization)
			}
			engine.ServeHTTP(recorder, request)

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Equal(t, tt.wantHandled, handled)
			entry := assertAuthLog(t, logs, tt.wantLogLevel, tt.wantLogMsg)
			if tt.wantLogMsg != "" {
				assertAuthFailureFields(t, entry.ContextMap(), tt.path)
				if tt.wantMismatch {
					fields := entry.ContextMap()
					require.Equal(t, authTestUserID, fields["user_id"])
					require.Equal(t, int64(3), fields["current_token_version"])
					require.Equal(t, int64(1), fields["token_version"])
				}
			}
			if tt.wantCode != 0 {
				var envelope response.Envelope
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
				wantMessage := response.MessageAuthInvalid
				if tt.wantStatus == http.StatusInternalServerError {
					wantMessage = response.MessageInternalError
				}
				require.False(t, envelope.Success)
				require.Equal(t, tt.wantCode, envelope.Code)
				require.Equal(t, wantMessage, envelope.Message)
			}
		})
	}
}

func TestAuthMiddlewareExpiredTokenDoesNotCallVersionValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expiredToken := signAuthTestToken(t, "secret", authTestUserID, 1, "s-123", time.Now().Add(-time.Hour))
	validator := NewMockTokenVersionValidator(gomock.NewController(t))
	engine := gin.New()
	engine.Use(AuthWithTokenVersionValidator(zap.NewNop(), testAccessTokenVerifier{secret: "secret"}, validator))
	engine.GET("/*path", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set(auth.AuthorizationHeader, auth.TokenPrefix+expiredToken)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func assertAuthLog(t *testing.T, logs *observer.ObservedLogs, wantLevel zapcore.Level, wantMsg string) observer.LoggedEntry {
	t.Helper()
	entries := logs.All()
	if wantMsg == "" {
		require.Empty(t, entries)
		return observer.LoggedEntry{}
	}

	for _, entry := range entries {
		require.False(t, wantLevel < zapcore.ErrorLevel && entry.Level >= zapcore.ErrorLevel, "unexpected error log: level=%s msg=%q", entry.Level, entry.Message)
	}

	var found observer.LoggedEntry
	foundEntry := false
	for _, entry := range entries {
		if entry.Level == wantLevel && entry.Message == wantMsg {
			found = entry
			foundEntry = true
			break
		}
	}
	require.True(t, foundEntry, "missing log level=%s msg=%q in %#v", wantLevel, wantMsg, entries)
	return found
}

func assertAuthFailureFields(t *testing.T, fields map[string]any, rawPath string) {
	t.Helper()
	require.Equal(t, http.MethodGet, fields["method"])
	require.Equal(t, "/*path", fields["path"])
	require.NotEqual(t, rawPath, fields["path"])
	require.Equal(t, "auth-test-agent", fields["user_agent"])
	require.Equal(t, "203.0.113.10", fields["client_ip"])
}

func signAuthTestToken(t *testing.T, secret, userID string, tokenVersion int64, sessionID string, expiresAt time.Time) string {
	return signAuthSubjectTestToken(t, secret, "access", userID, tokenVersion, sessionID, expiresAt)
}

func signAuthSubjectTestToken(t *testing.T, secret, subject, userID string, tokenVersion int64, sessionID string, expiresAt time.Time) string {
	t.Helper()
	tokenID, err := runtimeid.NewUUID()
	require.NoError(t, err)
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, testAuthClaims{
		UserID:           userID,
		TokenVersion:     tokenVersion,
		SessionID:        sessionID,
		RegisteredClaims: jwtv5.RegisteredClaims{ID: tokenID.String(), Subject: subject, ExpiresAt: jwtv5.NewNumericDate(expiresAt)},
	}).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}

type testAuthClaims struct {
	UserID       string `json:"user_id"`
	TokenVersion int64  `json:"token_version"`
	SessionID    string `json:"session_id"`
	jwtv5.RegisteredClaims
}

type testAccessTokenVerifier struct {
	secret string
}

func (v testAccessTokenVerifier) VerifyAccessToken(tokenString string) (auth.AccessToken, error) {
	if v.secret == "" {
		return auth.AccessToken{}, auth.ErrMissingSecret
	}
	claims := &testAuthClaims{}
	token, err := jwtv5.ParseWithClaims(tokenString, claims, func(_ *jwtv5.Token) (any, error) {
		return []byte(v.secret), nil
	}, jwtv5.WithExpirationRequired())
	if err != nil {
		return auth.AccessToken{}, err
	}
	if token == nil || !token.Valid || claims.Subject != "access" || claims.TokenVersion <= 0 || claims.SessionID == "" {
		return auth.AccessToken{}, jwtv5.ErrTokenInvalidClaims
	}
	return auth.AccessToken{UserID: claims.UserID, SessionID: claims.SessionID, TokenVersion: claims.TokenVersion}, nil
}
