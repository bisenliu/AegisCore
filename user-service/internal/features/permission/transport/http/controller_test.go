package permissionhttp

import (
	"bytes"
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
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissioncommand "github.com/aegiscore/user-service/internal/features/permission/application/command"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	permissionHTTPTestPermissionID       = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
	permissionHTTPTestSecondPermissionID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f"
	permissionHTTPTestUserID             = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d50"
	permissionHTTPTestCursorID           = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d51"
)

var (
	permissionHTTPTestPermissionUUID       = uuid.MustParse(permissionHTTPTestPermissionID)
	permissionHTTPTestSecondPermissionUUID = uuid.MustParse(permissionHTTPTestSecondPermissionID)
	permissionHTTPTestUserUUID             = uuid.MustParse(permissionHTTPTestUserID)
	permissionHTTPTestCursorUUID           = uuid.MustParse(permissionHTTPTestCursorID)
)

type permissionHTTPTestEnvelope struct {
	Success bool                `json:"success"`
	Code    contracterrors.Code `json:"code"`
	Message string              `json:"message"`
	Data    json.RawMessage     `json:"data,omitempty"`
	Errors  json.RawMessage     `json:"errors,omitempty"`
}

func TestPermissionControllerListPermissions(t *testing.T) {
	t.Run("success with filters and pagination", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		permission := permissionHTTPTestPermission(permissionHTTPTestPermissionUUID, "查询用户")
		var gotQuery permissionquery.ListPermissionsQuery
		queries.EXPECT().ListPermissions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query permissionquery.ListPermissionsQuery) (*permissionquery.ListPermissionsResult, error) {
			gotQuery = query
			return &permissionquery.ListPermissionsResult{Items: []permissiondomain.Permission{permission}, PageSize: 100, NextCursor: permissionHTTPTestPermissionID, HasNext: true}, nil
		})

		target := "/api/v1/permissions?cursor=" + permissionHTTPTestCursorID + "&page_size=200&module=%20user%20&http_method=%20GET%20&active=true&system=false"
		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, target, nil)
		envelope := expectPermissionEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodePermissionHTTPData[PermissionListResponseDoc](t, envelope)

		require.NotNil(t, gotQuery.Cursor)
		require.Equal(t, permissionHTTPTestCursorUUID, *gotQuery.Cursor)
		require.Equal(t, 100, gotQuery.PageSize)
		require.Equal(t, 100, gotQuery.Limit)
		require.Equal(t, "user", gotQuery.Module)
		require.Equal(t, "GET", gotQuery.HTTPMethod)
		require.NotNil(t, gotQuery.Active)
		require.Equal(t, true, *gotQuery.Active)
		require.NotNil(t, gotQuery.IsSystem)
		require.Equal(t, false, *gotQuery.IsSystem)
		require.Len(t, payload.Items, 1)
		assertPermissionHTTPResponse(t, permission, payload.Items[0])
		require.Equal(t, 100, payload.Pagination.PageSize)
		require.Equal(t, permissionHTTPTestPermissionID, payload.Pagination.NextCursor)
		require.Equal(t, true, payload.Pagination.HasNext)
	})

	t.Run("invalid cursor is rejected before query service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions?cursor=bad", nil)
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeBadRequest, messages.InvalidPermission)
	})

	t.Run("invalid query type is rejected before query service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions?active=maybe", nil)
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeBadRequest, "")
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		queries.EXPECT().ListPermissions(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions", nil)
		expectPermissionEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestPermissionControllerCreatePermission(t *testing.T) {
	t.Run("success trims fields and returns created envelope", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		permission := permissionHTTPTestPermission(permissionHTTPTestPermissionUUID, "查询用户")
		active := false
		var gotCommand permissioncommand.CreatePermissionCommand
		commands.EXPECT().CreatePermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd permissioncommand.CreatePermissionCommand) (*permissioncommand.PermissionResult, error) {
			gotCommand = cmd
			return &permissioncommand.PermissionResult{Permission: permission}, nil
		})

		body := `{"name":" 查询用户 ","description":" 查询用户列表 ","module":" user ","http_method":" GET ","path_template":" /api/v1/users ","active":false,"system":true}`
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions", jsonBody(body))
		envelope := expectPermissionEnvelope(t, recorder, http.StatusCreated, true, contracterrors.CodeOK, contractresponse.MessageCreated)
		payload := decodePermissionHTTPData[PermissionResponse](t, envelope)

		require.Equal(t, "查询用户", gotCommand.Name)
		require.Equal(t, "查询用户列表", gotCommand.Description)
		require.Equal(t, "user", gotCommand.Module)
		require.Equal(t, "GET", gotCommand.HTTPMethod)
		require.Equal(t, "/api/v1/users", gotCommand.PathTemplate)
		require.NotNil(t, gotCommand.Active)
		require.Equal(t, active, *gotCommand.Active)
		require.Equal(t, true, gotCommand.IsSystem)
		assertPermissionHTTPResponse(t, permission, payload)
	})

	t.Run("empty body is rejected before command service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions", jsonBody(""))
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeBadRequest, validation.ErrEmptyRequestBody)
	})

	t.Run("validation failure is rejected before command service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions", jsonBody(`{"description":"missing required fields"}`))
		envelope := expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
		require.NotEmpty(t, envelope.Errors)
	})

	t.Run("permission conflict maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		commands.EXPECT().CreatePermission(gomock.Any(), gomock.Any()).Return(nil, permissiondomain.ErrPermissionAlreadyExists)

		body := `{"name":"查询用户","module":"user","http_method":"GET","path_template":"/api/v1/users"}`
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions", jsonBody(body))
		expectPermissionEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.PermissionAlreadyExists)
	})

	t.Run("domain validation error maps to validation envelope", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		commands.EXPECT().CreatePermission(gomock.Any(), gomock.Any()).Return(nil, permissiondomain.ErrPermissionInvalid)

		body := `{"name":"查询用户","module":"user","http_method":"GET","path_template":"/api/v1/users"}`
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions", jsonBody(body))
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, messages.InvalidPermission)
	})
}

func TestPermissionControllerGetPermission(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		permission := permissionHTTPTestPermission(permissionHTTPTestPermissionUUID, "查询用户")
		var gotQuery permissionquery.GetPermissionQuery
		queries.EXPECT().GetPermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query permissionquery.GetPermissionQuery) (*permissionquery.PermissionResult, error) {
			gotQuery = query
			return &permissionquery.PermissionResult{Permission: permission}, nil
		})

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/"+permissionHTTPTestPermissionID, nil)
		envelope := expectPermissionEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodePermissionHTTPData[PermissionResponse](t, envelope)

		require.Equal(t, permissionHTTPTestPermissionUUID, gotQuery.PermissionID)
		assertPermissionHTTPResponse(t, permission, payload)
	})

	t.Run("invalid permission id is rejected before query service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/bad", nil)
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("not found maps to not found envelope", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		queries.EXPECT().GetPermission(gomock.Any(), gomock.Any()).Return(nil, permissiondomain.ErrPermissionNotFound)

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/"+permissionHTTPTestPermissionID, nil)
		expectPermissionEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.PermissionNotFound)
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		queries.EXPECT().GetPermission(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/"+permissionHTTPTestPermissionID, nil)
		expectPermissionEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestPermissionControllerUpdatePermission(t *testing.T) {
	t.Run("success trims fields and returns no content", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		var gotCommand permissioncommand.UpdatePermissionCommand
		commands.EXPECT().UpdatePermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd permissioncommand.UpdatePermissionCommand) error {
			gotCommand = cmd
			return nil
		})

		body := `{"name":" 更新用户 ","description":" 更新用户资料 ","module":" user ","http_method":" PUT ","path_template":" /api/v1/users/:id ","active":false}`
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPut, "/api/v1/permissions/"+permissionHTTPTestPermissionID, jsonBody(body))
		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.Empty(t, recorder.Body.String())

		require.Equal(t, permissionHTTPTestPermissionUUID, gotCommand.PermissionID)
		require.Equal(t, "更新用户", gotCommand.Name)
		require.Equal(t, "更新用户资料", gotCommand.Description)
		require.Equal(t, "user", gotCommand.Module)
		require.Equal(t, "PUT", gotCommand.HTTPMethod)
		require.Equal(t, "/api/v1/users/:id", gotCommand.PathTemplate)
		require.Equal(t, false, gotCommand.Active)
	})

	t.Run("invalid permission id is rejected before command service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		body := `{"name":"更新用户","module":"user","http_method":"PUT","path_template":"/api/v1/users/:id","active":true}`
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPut, "/api/v1/permissions/bad", jsonBody(body))
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("validation failure is rejected before command service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPut, "/api/v1/permissions/"+permissionHTTPTestPermissionID, jsonBody(`{"description":"missing name","active":true}`))
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("protected system permission maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		commands.EXPECT().UpdatePermission(gomock.Any(), gomock.Any()).Return(permissiondomain.ErrSystemPermissionProtected)

		body := `{"name":"系统权限","module":"user","http_method":"PUT","path_template":"/api/v1/users/:id","active":true}`
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPut, "/api/v1/permissions/"+permissionHTTPTestPermissionID, jsonBody(body))
		expectPermissionEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.SystemPermissionProtected)
	})

	t.Run("command service error maps to internal error", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		commands.EXPECT().UpdatePermission(gomock.Any(), gomock.Any()).Return(errors.New("database down"))

		body := `{"name":"更新用户","module":"user","http_method":"PUT","path_template":"/api/v1/users/:id","active":true}`
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPut, "/api/v1/permissions/"+permissionHTTPTestPermissionID, jsonBody(body))
		expectPermissionEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestPermissionControllerSetPermissionActive(t *testing.T) {
	t.Run("enable success", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		var gotCommand permissioncommand.SetPermissionActiveCommand
		commands.EXPECT().EnablePermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd permissioncommand.SetPermissionActiveCommand) error {
			gotCommand = cmd
			return nil
		})

		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions/"+permissionHTTPTestPermissionID+"/enable", nil)
		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.Empty(t, recorder.Body.String())

		require.Equal(t, permissionHTTPTestPermissionUUID, gotCommand.PermissionID)
	})

	t.Run("disable success", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		var gotCommand permissioncommand.SetPermissionActiveCommand
		commands.EXPECT().DisablePermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd permissioncommand.SetPermissionActiveCommand) error {
			gotCommand = cmd
			return nil
		})

		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions/"+permissionHTTPTestPermissionID+"/disable", nil)
		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.Empty(t, recorder.Body.String())

		require.Equal(t, permissionHTTPTestPermissionUUID, gotCommand.PermissionID)
	})

	t.Run("invalid permission id is rejected before command service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions/bad/enable", nil)
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("enable not found maps to not found envelope", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		commands.EXPECT().EnablePermission(gomock.Any(), gomock.Any()).Return(permissiondomain.ErrPermissionNotFound)

		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions/"+permissionHTTPTestPermissionID+"/enable", nil)
		expectPermissionEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.PermissionNotFound)
	})

	t.Run("disable command service error maps to internal error", func(t *testing.T) {
		engine, commands, _ := newPermissionHTTPTestHarness(t)
		commands.EXPECT().DisablePermission(gomock.Any(), gomock.Any()).Return(errors.New("database down"))

		recorder := performPermissionHTTPRequest(t, engine, http.MethodPost, "/api/v1/permissions/"+permissionHTTPTestPermissionID+"/disable", nil)
		expectPermissionEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestPermissionControllerListUserEffectivePermissions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		permission := permissionHTTPTestPermission(permissionHTTPTestPermissionUUID, "查询用户")
		var gotQuery permissionquery.UserEffectivePermissionsQuery
		queries.EXPECT().ListUserEffectivePermissions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query permissionquery.UserEffectivePermissionsQuery) (*permissionquery.UserEffectivePermissionsResult, error) {
			gotQuery = query
			return &permissionquery.UserEffectivePermissionsResult{Items: []permissiondomain.Permission{permission}}, nil
		})

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/users/"+permissionHTTPTestUserID+"/effective", nil)
		envelope := expectPermissionEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodePermissionHTTPData[[]PermissionResponse](t, envelope)

		require.Equal(t, permissionHTTPTestUserUUID, gotQuery.UserID)
		require.Len(t, payload, 1)
		assertPermissionHTTPResponse(t, permission, payload[0])
	})

	t.Run("invalid user id is rejected before query service", func(t *testing.T) {
		engine, _, _ := newPermissionHTTPTestHarness(t)
		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/users/bad/effective", nil)
		expectPermissionEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		queries.EXPECT().ListUserEffectivePermissions(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/users/"+permissionHTTPTestUserID+"/effective", nil)
		expectPermissionEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestPermissionControllerGetRouteDiff(t *testing.T) {
	t.Run("success maps missing routes and stale permissions", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		stale := permissionHTTPTestPermission(permissionHTTPTestSecondPermissionUUID, "过期权限")
		queries.EXPECT().GetRouteDiff(gomock.Any()).Return(&permissionquery.RouteDiffResult{
			MissingInPermissions: []permissionapplication.DiscoveredRoute{{Method: "GET", Path: "/api/v1/users"}},
			StalePermissions:     []permissiondomain.Permission{stale},
		}, nil)

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/route-diff", nil)
		envelope := expectPermissionEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodePermissionHTTPData[RouteDiffResponse](t, envelope)

		require.Len(t, payload.MissingInPermissions, 1)
		require.Equal(t, "GET", payload.MissingInPermissions[0].HTTPMethod)
		require.Equal(t, "/api/v1/users", payload.MissingInPermissions[0].Path)
		require.Len(t, payload.StalePermissions, 1)
		assertPermissionHTTPResponse(t, stale, payload.StalePermissions[0])
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newPermissionHTTPTestHarness(t)
		queries.EXPECT().GetRouteDiff(gomock.Any()).Return(nil, errors.New("scanner down"))

		recorder := performPermissionHTTPRequest(t, engine, http.MethodGet, "/api/v1/permissions/route-diff", nil)
		expectPermissionEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func newPermissionHTTPTestHarness(t *testing.T) (*gin.Engine, *MockPermissionCommandService, *MockPermissionQueryService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	commands := NewMockPermissionCommandService(ctrl)
	queries := NewMockPermissionQueryService(ctrl)
	controller := NewPermissionController(commands, queries, validator)
	engine := gin.New()
	RegisterRoutes(engine.Group("/api/v1/permissions"), controller)
	return engine, commands, queries
}

func performPermissionHTTPRequest(t *testing.T, engine *gin.Engine, method string, target string, body *string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload = []byte(*body)
	}
	request := httptest.NewRequest(method, target, bytes.NewReader(payload))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func jsonBody(body string) *string {
	return &body
}

func expectPermissionEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, status int, success bool, code contracterrors.Code, message string) permissionHTTPTestEnvelope {
	t.Helper()
	require.Equal(t, status, recorder.Code, "body=%s", recorder.Body.String())
	var envelope permissionHTTPTestEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, success, envelope.Success)
	require.Equal(t, code, envelope.Code)
	if message != "" {
		require.Equal(t, message, envelope.Message)
	}
	return envelope
}

func decodePermissionHTTPData[T any](t *testing.T, envelope permissionHTTPTestEnvelope) T {
	t.Helper()
	require.NotEmpty(t, envelope.Data)
	var data T
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	return data
}

func permissionHTTPTestPermission(permissionID uuid.UUID, name string) permissiondomain.Permission {
	return permissiondomain.Permission{
		ID:           1,
		PermissionID: permissionID,
		Name:         name,
		Description:  name + "说明",
		Module:       "user",
		HTTPMethod:   "GET",
		PathTemplate: "/api/v1/users",
		Active:       true,
		IsSystem:     false,
		CreatedAt:    1780048800000,
		UpdatedAt:    1780052400000,
	}
}

func assertPermissionHTTPResponse(t *testing.T, want permissiondomain.Permission, got PermissionResponse) {
	t.Helper()
	require.Equal(t, want.PermissionID.String(), got.PermissionID)
	require.Equal(t, want.Name, got.Name)
	require.Equal(t, want.Description, got.Description)
	require.Equal(t, want.Module, got.Module)
	require.Equal(t, want.HTTPMethod, got.HTTPMethod)
	require.Equal(t, want.PathTemplate, got.PathTemplate)
	require.Equal(t, want.Active, got.Active)
	require.Equal(t, want.IsSystem, got.System)
	require.Equal(t, want.CreatedAt, got.CreatedAt)
	require.Equal(t, want.UpdatedAt, got.UpdatedAt)
}
