package authhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonmw "github.com/aegiscore/common/http/middleware"
	commonauth "github.com/aegiscore/common/security/auth"
)

func TestAuthEntrypointsRejectOversizedRequestBodies(t *testing.T) {
	const maxBytes int64 = 64
	endpoints := []struct {
		name      string
		path      string
		validBody string
		handler   func(*AuthController) gin.HandlerFunc
		authorize bool
	}{
		{name: "login", path: "/api/v1/auth/login", validBody: `{"username":"alice","password":"secret"}`, handler: func(ctl *AuthController) gin.HandlerFunc { return ctl.LoginUser }},
		{name: "refresh", path: "/api/v1/auth/refresh", validBody: `{"refresh_token":"refresh-token"}`, handler: func(ctl *AuthController) gin.HandlerFunc { return ctl.RefreshToken }},
		{name: "force change password", path: "/api/v1/auth/force-change-password", validBody: `{"new_password":"new-secret-123"}`, handler: func(ctl *AuthController) gin.HandlerFunc { return ctl.ForceChangePassword }, authorize: true},
	}
	loads := []struct {
		name    string
		body    func(string) string
		chunked bool
	}{
		{name: "fixed length", body: func(string) string { return `{"padding":"` + strings.Repeat("x", 80) + `"}` }},
		{name: "chunked", body: func(string) string { return `{"padding":"` + strings.Repeat("x", 80) + `"}` }, chunked: true},
		{name: "oversized trailing json", body: func(valid string) string { return valid + ` {"padding":"` + strings.Repeat("x", 80) + `"}` }, chunked: true},
	}

	for _, endpoint := range endpoints {
		for _, load := range loads {
			t.Run(endpoint.name+"/"+load.name, func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				ctl, _ := newTestAuthController(t)
				limit, err := commonmw.RequestBodyLimit(maxBytes)
				require.NoError(t, err)
				engine := gin.New()
				engine.Use(limit)
				engine.POST(endpoint.path, endpoint.handler(ctl))

				request := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(load.body(endpoint.validBody)))
				request.Header.Set("Content-Type", "application/json")
				if endpoint.authorize {
					request.Header.Set(commonauth.AuthorizationHeader, commonauth.TokenPrefix+"password-token")
				}
				if load.chunked {
					request.ContentLength = -1
				}
				recorder := httptest.NewRecorder()

				engine.ServeHTTP(recorder, request)

				status, envelope := decodeAuthEnvelope(t, recorder)
				require.Equal(t, http.StatusRequestEntityTooLarge, status)
				require.False(t, envelope.Success)
				require.Equal(t, contracterrors.CodeRequestBodyTooLarge, envelope.Code)
				require.Equal(t, contracterrors.MessageRequestBodyTooLarge, envelope.Message)
			})
		}
	}
}
