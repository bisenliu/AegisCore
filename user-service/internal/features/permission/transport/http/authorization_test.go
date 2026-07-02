package permissionhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonauth "github.com/aegiscore/common/security/auth"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
)

const authorizationTestUserID = "018f0000-0000-7000-8000-000000000801"

func TestAuthorizeAllowsRequestAndUsesFullPath(t *testing.T) {
	engine, authz := newAuthorizationTestEngine(t)
	authz.EXPECT().Enforce(gomock.Any(), authorizationTestUserID, "/api/v1/users/:user_id", http.MethodGet).Return(true, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestAuthorizeReadsUserIDFromRequestContext(t *testing.T) {
	authz := NewMockAuthorizer(gomock.NewController(t))
	authz.EXPECT().Enforce(gomock.Any(), authorizationTestUserID, "/api/v1/users/:user_id", http.MethodGet).Return(true, nil)
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
			authz := NewMockAuthorizer(gomock.NewController(t))
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
		})
	}
}

func TestAuthorizeDeniedAndErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		allowed    bool
		err        error
		wantStatus int
		wantCode   contracterrors.Code
	}{
		{name: "denied", allowed: false, wantStatus: http.StatusForbidden, wantCode: contracterrors.CodeForbidden},
		{name: "error", err: errors.New("engine unavailable"), wantStatus: http.StatusInternalServerError, wantCode: contracterrors.CodeInternalError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, authz := newAuthorizationTestEngine(t)
			authz.EXPECT().Enforce(gomock.Any(), authorizationTestUserID, "/api/v1/users/:user_id", http.MethodGet).Return(tt.allowed, tt.err)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)

			engine.ServeHTTP(response, request)

			assertAuthorizationEnvelope(t, response, tt.wantStatus, tt.wantCode)
		})
	}
}

func TestAuthorizeMapsInvalidSubjectToUnauthenticated(t *testing.T) {
	authz := NewMockAuthorizer(gomock.NewController(t))
	authz.EXPECT().Enforce(gomock.Any(), "not-a-uuid", "/api/v1/users/:user_id", http.MethodGet).Return(false, permissionauthorization.ErrInvalidSubjectUserID)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) { c.Set(commonauth.UserIDKey, "not-a-uuid") }, Authorize(authz), func(c *gin.Context) { c.Status(http.StatusOK) })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)

	engine.ServeHTTP(response, request)

	assertAuthorizationEnvelope(t, response, http.StatusUnauthorized, contracterrors.CodeUnauthenticated)
}

func TestAuthorizeDeniesRBACNegativeScenarios(t *testing.T) {
	tests := []string{
		"user has no roles",
		"role disabled",
		"permission disabled",
		"user role unbound",
		"role permission unbound",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			engine, authz := newAuthorizationTestEngine(t)
			authz.EXPECT().Enforce(gomock.Any(), authorizationTestUserID, "/api/v1/users/:user_id", http.MethodGet).Return(false, nil)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorizationTestUserID, nil)

			engine.ServeHTTP(response, request)

			assertAuthorizationEnvelope(t, response, http.StatusForbidden, contracterrors.CodeForbidden)
		})
	}
}

func TestAuthorizeAllowsSuperAdminWildcardDecision(t *testing.T) {
	engine, authz := newAuthorizationTestEngine(t)
	authz.EXPECT().Enforce(gomock.Any(), authorizationTestUserID, "/api/v1/users/:user_id", http.MethodDelete).Return(true, nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+authorizationTestUserID, nil)

	engine.DELETE("/api/v1/users/:user_id", func(c *gin.Context) { c.Set(commonauth.UserIDKey, authorizationTestUserID) }, Authorize(authz), func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
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
			authz := NewMockAuthorizer(gomock.NewController(t))
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.Handle(tt.method, "/api/v1/users/:user_id", func(c *gin.Context) { c.Set(commonauth.UserIDKey, authorizationTestUserID) }, Authorize(authz, tt.options...), func(c *gin.Context) { c.Status(http.StatusOK) })
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, "/api/v1/users/"+authorizationTestUserID, nil)

			engine.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}
		})
	}
}

func newAuthorizationTestEngine(t *testing.T) (*gin.Engine, *MockAuthorizer) {
	t.Helper()
	authz := NewMockAuthorizer(gomock.NewController(t))
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
