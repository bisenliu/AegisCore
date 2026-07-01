package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	commoncasbin "github.com/aegiscore/common/security/casbin"
)

func TestCasbinAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		authorizer *recordingCasbinAuthorizer
		resolver   CasbinRequestResolver
		opts       []CasbinAuthorizationOption
		wantStatus int
		wantCalls  int
	}{
		{name: "allowed", authorizer: &recordingCasbinAuthorizer{}, resolver: testCasbinResolver, wantStatus: http.StatusOK, wantCalls: 1},
		{name: "denied", authorizer: &recordingCasbinAuthorizer{err: commoncasbin.ErrDenied}, resolver: testCasbinResolver, wantStatus: http.StatusForbidden, wantCalls: 1},
		{name: "resolver error", authorizer: &recordingCasbinAuthorizer{}, resolver: func(*gin.Context) (commoncasbin.Request, error) {
			return commoncasbin.Request{}, errors.New("missing subject")
		}, wantStatus: http.StatusInternalServerError},
		{name: "whitelist", authorizer: &recordingCasbinAuthorizer{err: commoncasbin.ErrDenied}, resolver: testCasbinResolver, opts: []CasbinAuthorizationOption{WithCasbinAuthorizationWhitelist(CasbinAuthorizationWhitelistRule{Method: http.MethodGet, PathTemplate: "/api/v1/users/:user_id"})}, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.GET("/api/v1/users/:user_id", CasbinAuthorization(tt.authorizer, tt.resolver, tt.opts...), func(c *gin.Context) { c.Status(http.StatusOK) })
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)

			engine.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if tt.authorizer != nil && tt.authorizer.calls != tt.wantCalls {
				t.Fatalf("authorizer calls = %d, want %d", tt.authorizer.calls, tt.wantCalls)
			}
		})
	}
}

func TestCasbinAuthorizationUsesCustomErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", CasbinAuthorization(
		&recordingCasbinAuthorizer{err: commoncasbin.ErrDenied},
		testCasbinResolver,
		WithCasbinAuthorizationErrorHandler(func(c *gin.Context, _ error) {
			c.AbortWithStatus(http.StatusTeapot)
		}),
	), func(c *gin.Context) { c.Status(http.StatusOK) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTeapot)
	}
}

func testCasbinResolver(c *gin.Context) (commoncasbin.Request, error) {
	return commoncasbin.Request{Subject: "user:1", Object: c.FullPath(), Action: c.Request.Method}, nil
}

type recordingCasbinAuthorizer struct {
	err   error
	calls int
	req   commoncasbin.Request
}

func (a *recordingCasbinAuthorizer) Authorize(_ context.Context, req commoncasbin.Request) error {
	a.calls++
	a.req = req
	return a.err
}
