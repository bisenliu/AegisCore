package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	commoncasbin "github.com/aegiscore/common/security/casbin"
)

func TestCasbinAuthorization(t *testing.T) {
	tests := []struct {
		name            string
		setupAuthorizer func(ctrl *gomock.Controller) CasbinAuthorizer
		resolver        CasbinRequestResolver
		opts            []CasbinAuthorizationOption
		wantStatus      int
	}{
		{name: "allowed", setupAuthorizer: func(ctrl *gomock.Controller) CasbinAuthorizer {
			authorizer := NewMockCasbinAuthorizer(ctrl)
			authorizer.EXPECT().Authorize(gomock.Any(), testCasbinRequest()).Return(nil)
			return authorizer
		}, resolver: testCasbinResolver, wantStatus: http.StatusOK},
		{name: "denied", setupAuthorizer: func(ctrl *gomock.Controller) CasbinAuthorizer {
			authorizer := NewMockCasbinAuthorizer(ctrl)
			authorizer.EXPECT().Authorize(gomock.Any(), testCasbinRequest()).Return(commoncasbin.ErrDenied)
			return authorizer
		}, resolver: testCasbinResolver, wantStatus: http.StatusForbidden},
		{name: "resolver error", setupAuthorizer: func(ctrl *gomock.Controller) CasbinAuthorizer {
			return NewMockCasbinAuthorizer(ctrl)
		}, resolver: func(*gin.Context) (commoncasbin.Request, error) {
			return commoncasbin.Request{}, errors.New("missing subject")
		}, wantStatus: http.StatusInternalServerError},
		{name: "whitelist", setupAuthorizer: func(ctrl *gomock.Controller) CasbinAuthorizer {
			return NewMockCasbinAuthorizer(ctrl)
		}, resolver: testCasbinResolver, opts: []CasbinAuthorizationOption{WithCasbinAuthorizationWhitelist(CasbinAuthorizationWhitelistRule{Method: http.MethodGet, PathTemplate: "/api/v1/users/:user_id"})}, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			authorizer := tt.setupAuthorizer(gomock.NewController(t))
			engine := gin.New()
			engine.GET("/api/v1/users/:user_id", CasbinAuthorization(authorizer, tt.resolver, tt.opts...), func(c *gin.Context) { c.Status(http.StatusOK) })
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)

			engine.ServeHTTP(recorder, request)

			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
		})
	}
}

func TestCasbinAuthorizationUsesCustomErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authorizer := NewMockCasbinAuthorizer(gomock.NewController(t))
	authorizer.EXPECT().Authorize(gomock.Any(), testCasbinRequest()).Return(commoncasbin.ErrDenied)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", CasbinAuthorization(
		authorizer,
		testCasbinResolver,
		WithCasbinAuthorizationErrorHandler(func(c *gin.Context, _ error) {
			c.AbortWithStatus(http.StatusTeapot)
		}),
	), func(c *gin.Context) { c.Status(http.StatusOK) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusTeapot, recorder.Code)
}

func testCasbinRequest() commoncasbin.Request {
	return commoncasbin.Request{Subject: "user:1", Object: "/api/v1/users/:user_id", Action: http.MethodGet}
}

func testCasbinResolver(c *gin.Context) (commoncasbin.Request, error) {
	return commoncasbin.Request{Subject: "user:1", Object: c.FullPath(), Action: c.Request.Method}, nil
}
