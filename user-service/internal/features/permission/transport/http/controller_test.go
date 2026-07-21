package permissionhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/validation"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

const (
	permissionHTTPTestPermissionID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
	permissionHTTPTestUserID       = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d50"
)

type permissionHTTPTestEnvelope struct {
	Success bool                `json:"success"`
	Code    contracterrors.Code `json:"code"`
	Message string              `json:"message"`
	Data    json.RawMessage     `json:"data,omitempty"`
}

func TestPermissionControllerListPermissions(t *testing.T) {
	engine, queries := newPermissionHTTPTestHarness(t)
	permission := permissionHTTPTestPermission()
	queries.EXPECT().ListPermissions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query permissionquery.ListPermissionsQuery) (*permissionquery.ListPermissionsResult, error) {
		require.Equal(t, "user", query.Module)
		require.Equal(t, http.MethodGet, query.HTTPMethod)
		return &permissionquery.ListPermissionsResult{Items: []permissiondomain.Permission{permission}, PageSize: 20}, nil
	})

	recorder := performPermissionHTTPRequest(engine, http.MethodGet, "/api/v1/permissions?module=user&http_method=GET")
	envelope := expectPermissionEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	var payload PermissionListResponseDoc
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	require.Len(t, payload.Items, 1)
	assertPermissionHTTPResponse(t, permission, payload.Items[0])
}

func TestPermissionControllerListUserEffectivePermissions(t *testing.T) {
	engine, queries := newPermissionHTTPTestHarness(t)
	permission := permissionHTTPTestPermission()
	queries.EXPECT().ListUserEffectivePermissions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query permissionquery.UserEffectivePermissionsQuery) (*permissionquery.UserEffectivePermissionsResult, error) {
		require.Equal(t, uuid.MustParse(permissionHTTPTestUserID), query.UserID)
		return &permissionquery.UserEffectivePermissionsResult{Items: []permissiondomain.Permission{permission}}, nil
	})

	recorder := performPermissionHTTPRequest(engine, http.MethodGet, "/api/v1/permissions/users/"+permissionHTTPTestUserID+"/effective")
	envelope := expectPermissionEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK)
	var payload []PermissionResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	require.Len(t, payload, 1)
	assertPermissionHTTPResponse(t, permission, payload[0])
}

func TestPermissionControllerQueryFailure(t *testing.T) {
	engine, queries := newPermissionHTTPTestHarness(t)
	queries.EXPECT().ListPermissions(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))
	recorder := performPermissionHTTPRequest(engine, http.MethodGet, "/api/v1/permissions")
	expectPermissionEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError)
}

func TestPermissionRoutesOnlyExposeReadProjection(t *testing.T) {
	engine, _ := newPermissionHTTPTestHarness(t)
	want := map[string]bool{
		http.MethodGet + " /api/v1/permissions":                          true,
		http.MethodGet + " /api/v1/permissions/users/:user_id/effective": true,
	}
	routes := engine.Routes()
	require.Len(t, routes, len(want))
	for _, route := range routes {
		require.True(t, want[route.Method+" "+route.Path], route.Method+" "+route.Path)
	}
}

func newPermissionHTTPTestHarness(t *testing.T) (*gin.Engine, *MockPermissionQueryService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	queries := NewMockPermissionQueryService(gomock.NewController(t))
	engine := gin.New()
	RegisterRoutes(engine.Group("/api/v1/permissions"), NewPermissionController(queries, validator))
	return engine, queries
}

func performPermissionHTTPRequest(engine *gin.Engine, method, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func expectPermissionEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, status int, success bool, code contracterrors.Code) permissionHTTPTestEnvelope {
	t.Helper()
	require.Equal(t, status, recorder.Code, recorder.Body.String())
	var envelope permissionHTTPTestEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, success, envelope.Success)
	require.Equal(t, code, envelope.Code)
	if status == http.StatusOK {
		require.Equal(t, contractresponse.MessageOK, envelope.Message)
	}
	return envelope
}

func permissionHTTPTestPermission() permissiondomain.Permission {
	return permissiondomain.Permission{ID: 1, PermissionID: uuid.MustParse(permissionHTTPTestPermissionID), Name: "查询用户", Description: "查询用户说明", Module: "user", HTTPMethod: http.MethodGet, PathTemplate: "/api/v1/users", CreatedAt: 1780048800000, UpdatedAt: 1780052400000}
}

func assertPermissionHTTPResponse(t *testing.T, want permissiondomain.Permission, got PermissionResponse) {
	t.Helper()
	require.Equal(t, want.PermissionID.String(), got.PermissionID)
	require.Equal(t, want.Name, got.Name)
	require.Equal(t, want.Description, got.Description)
	require.Equal(t, want.Module, got.Module)
	require.Equal(t, want.HTTPMethod, got.HTTPMethod)
	require.Equal(t, want.PathTemplate, got.PathTemplate)
	require.Equal(t, want.CreatedAt, got.CreatedAt)
	require.Equal(t, want.UpdatedAt, got.UpdatedAt)
}
