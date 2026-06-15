package permissionhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/gin-gonic/gin"
)

const authorizationTestUserID = "018f0000-0000-7000-8000-000000000801"

func TestAuthorizeAllowsRequestAndUsesFullPath(t *testing.T) {
	engine, authz := newAuthorizationTestEngine(&fakeAuthorizer{allowed: true})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if authz.calls != 1 || authz.userID != authorizationTestUserID || authz.pathTemplate != "/api/v1/users/:user_id" || authz.method != http.MethodGet {
		t.Fatalf("authorizer call = %#v", authz)
	}
}

func TestAuthorizeReadsUserIDFromRequestContext(t *testing.T) {
	authz := &fakeAuthorizer{allowed: true}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", Authorize(authz), func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)
	request = request.WithContext(commonauth.WithUserID(context.Background(), authorizationTestUserID))
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if authz.userID != authorizationTestUserID {
		t.Fatalf("authorizer userID = %q, want %q", authz.userID, authorizationTestUserID)
	}
}

func TestAuthorizeRejectsMissingOrInvalidUserID(t *testing.T) {
	tests := []struct {
		name     string
		setUser  func(*gin.Context)
		wantCode contracterrors.Code
	}{
		{name: "missing", wantCode: contracterrors.CodeUnauthenticated},
		{name: "invalid type", setUser: func(c *gin.Context) { c.Set(commonauth.UserIDKey, 123) }, wantCode: contracterrors.CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz := &fakeAuthorizer{allowed: true}
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
				if tt.setUser != nil {
					tt.setUser(c)
				}
			}, Authorize(authz), func(c *gin.Context) { c.Status(http.StatusOK) })
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)

			engine.ServeHTTP(response, request)

			assertAuthorizationEnvelope(t, response, http.StatusUnauthorized, tt.wantCode)
			if authz.calls != 0 {
				t.Fatalf("authorizer calls = %d, want 0", authz.calls)
			}
		})
	}
}

func TestAuthorizeDeniedAndErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		authz      *fakeAuthorizer
		wantStatus int
		wantCode   contracterrors.Code
	}{
		{name: "denied", authz: &fakeAuthorizer{allowed: false}, wantStatus: http.StatusForbidden, wantCode: contracterrors.CodeForbidden},
		{name: "error", authz: &fakeAuthorizer{err: errors.New("engine unavailable")}, wantStatus: http.StatusInternalServerError, wantCode: contracterrors.CodeInternalError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, _ := newAuthorizationTestEngine(tt.authz)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)

			engine.ServeHTTP(response, request)

			assertAuthorizationEnvelope(t, response, tt.wantStatus, tt.wantCode)
		})
	}
}

func TestAuthorizeWhitelistAndOptionsBypass(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		options []AuthorizationOption
	}{
		{name: "whitelist", method: http.MethodGet, options: []AuthorizationOption{WithAuthorizationWhitelist(AuthorizationWhitelistRule{Method: http.MethodGet, PathTemplate: "/api/v1/users/:user_id"})}},
		{name: "options", method: http.MethodOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz := &fakeAuthorizer{allowed: false}
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.Handle(tt.method, "/api/v1/users/:user_id", func(c *gin.Context) { c.Set(commonauth.UserIDKey, authorizationTestUserID) }, Authorize(authz, tt.options...), func(c *gin.Context) { c.Status(http.StatusOK) })
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, "/api/v1/users/"+authorizationTestUserID, nil)

			engine.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}
			if authz.calls != 0 {
				t.Fatalf("authorizer calls = %d, want 0", authz.calls)
			}
		})
	}
}

func newAuthorizationTestEngine(authz *fakeAuthorizer) (*gin.Engine, *fakeAuthorizer) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) { c.Set(commonauth.UserIDKey, authorizationTestUserID) }, Authorize(authz), func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine, authz
}

func assertAuthorizationEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode contracterrors.Code) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var envelope contractresponse.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != wantCode {
		t.Fatalf("envelope = %#v, want failure code %d", envelope, wantCode)
	}
}

type fakeAuthorizer struct {
	allowed      bool
	err          error
	calls        int
	userID       string
	pathTemplate string
	method       string
}

func (a *fakeAuthorizer) Enforce(_ context.Context, userID string, pathTemplate string, method string) (bool, error) {
	a.calls++
	a.userID = userID
	a.pathTemplate = pathTemplate
	a.method = method
	return a.allowed, a.err
}
