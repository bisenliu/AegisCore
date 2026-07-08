package rolehttp

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
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	roleHTTPTestRoleID       = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"
	roleHTTPTestSecondRoleID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f"
	roleHTTPTestPermissionID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d50"
	roleHTTPTestUserID       = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d51"
	roleHTTPTestCursorID     = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d52"
)

var (
	roleHTTPTestRoleUUID       = uuid.MustParse(roleHTTPTestRoleID)
	roleHTTPTestSecondRoleUUID = uuid.MustParse(roleHTTPTestSecondRoleID)
	roleHTTPTestPermissionUUID = uuid.MustParse(roleHTTPTestPermissionID)
	roleHTTPTestUserUUID       = uuid.MustParse(roleHTTPTestUserID)
	roleHTTPTestCursorUUID     = uuid.MustParse(roleHTTPTestCursorID)
)

type roleHTTPTestEnvelope struct {
	Success bool                `json:"success"`
	Code    contracterrors.Code `json:"code"`
	Message string              `json:"message"`
	Data    json.RawMessage     `json:"data,omitempty"`
	Errors  json.RawMessage     `json:"errors,omitempty"`
}

func TestRoleControllerListRoles(t *testing.T) {
	t.Run("success with filters and pagination", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		var gotQuery rolequery.ListRolesQuery
		role := roleHTTPTestRole(roleHTTPTestRoleUUID, "管理员")
		queries.EXPECT().ListRoles(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query rolequery.ListRolesQuery) (*rolequery.ListRolesResult, error) {
			gotQuery = query
			return &rolequery.ListRolesResult{Items: []roledomain.Role{role}, PageSize: 100, NextCursor: roleHTTPTestRoleID, HasNext: true}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles?cursor="+roleHTTPTestCursorID+"&page_size=200&active=true&system=false", nil)
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[RoleListResponseDoc](t, envelope)

		require.NotNil(t, gotQuery.Cursor)
		require.Equal(t, roleHTTPTestCursorUUID, *gotQuery.Cursor)
		require.Equal(t, 100, gotQuery.PageSize)
		require.Equal(t, 100, gotQuery.Limit)
		require.NotNil(t, gotQuery.Active)
		require.True(t, *gotQuery.Active)
		require.NotNil(t, gotQuery.IsSystem)
		require.False(t, *gotQuery.IsSystem)
		require.Len(t, payload.Items, 1)
		assertRoleHTTPResponse(t, role, payload.Items[0])
		require.Equal(t, 100, payload.Pagination.PageSize)
		require.Equal(t, roleHTTPTestRoleID, payload.Pagination.NextCursor)
		require.True(t, payload.Pagination.HasNext)
	})

	t.Run("invalid cursor is rejected before query service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles?cursor=bad", nil)
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeBadRequest, messages.InvalidRole)
	})

	t.Run("invalid query type is rejected before query service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles?active=maybe", nil)
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeBadRequest, "")
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		queries.EXPECT().ListRoles(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles", nil)
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestRoleControllerCreateRole(t *testing.T) {
	t.Run("success trims fields and returns created envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		role := roleHTTPTestRole(roleHTTPTestRoleUUID, "管理员")
		var gotCommand rolecommand.CreateRoleCommand
		commands.EXPECT().CreateRole(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.CreateRoleCommand) (*rolecommand.RoleResult, error) {
			gotCommand = cmd
			return &rolecommand.RoleResult{Role: role}, nil
		})

		body := `{"name":" 管理员 ","description":" 管理后台角色 ","active":false,"system":true}`
		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles", jsonBody(body))
		envelope := expectRoleEnvelope(t, recorder, http.StatusCreated, true, contracterrors.CodeOK, contractresponse.MessageCreated)
		payload := decodeRoleHTTPData[RoleResponse](t, envelope)

		require.Equal(t, "管理员", gotCommand.Name)
		require.Equal(t, "管理后台角色", gotCommand.Description)
		require.NotNil(t, gotCommand.Active)
		require.False(t, *gotCommand.Active)
		require.True(t, gotCommand.IsSystem)
		assertRoleHTTPResponse(t, role, payload)
	})

	t.Run("empty body is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles", jsonBody(""))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeBadRequest, validation.ErrEmptyRequestBody)
	})

	t.Run("validation failure is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles", jsonBody(`{"description":"missing name"}`))
		envelope := expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
		require.NotEmpty(t, envelope.Errors)
	})

	t.Run("role conflict maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().CreateRole(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRoleAlreadyExists)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles", jsonBody(`{"name":"管理员"}`))
		expectRoleEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.RoleAlreadyExists)
	})

	t.Run("domain validation maps to validation envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().CreateRole(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRoleInvalid)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles", jsonBody(`{"name":"管理员"}`))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, messages.InvalidRole)
	})

	t.Run("command service error maps to internal error", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().CreateRole(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles", jsonBody(`{"name":"管理员"}`))
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestRoleControllerGetRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		role := roleHTTPTestRole(roleHTTPTestRoleUUID, "管理员")
		var gotQuery rolequery.GetRoleQuery
		queries.EXPECT().GetRole(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query rolequery.GetRoleQuery) (*rolequery.RoleResult, error) {
			gotQuery = query
			return &rolequery.RoleResult{Role: role}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles/"+roleHTTPTestRoleID, nil)
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[RoleResponse](t, envelope)

		require.Equal(t, roleHTTPTestRoleUUID, gotQuery.RoleID)
		assertRoleHTTPResponse(t, role, payload)
	})

	t.Run("invalid role id is rejected before query service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles/bad", nil)
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("not found maps to not found envelope", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		queries.EXPECT().GetRole(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRoleNotFound)

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles/"+roleHTTPTestRoleID, nil)
		expectRoleEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.RoleNotFound)
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		queries.EXPECT().GetRole(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles/"+roleHTTPTestRoleID, nil)
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestRoleControllerUpdateRole(t *testing.T) {
	t.Run("success trims fields and returns no content", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		var gotCommand rolecommand.UpdateRoleCommand
		commands.EXPECT().UpdateRole(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.UpdateRoleCommand) error {
			gotCommand = cmd
			return nil
		})

		body := `{"name":" 审计员 ","description":" 审计角色 ","active":false}`
		recorder := performRoleHTTPRequest(t, engine, http.MethodPatch, "/api/v1/roles/"+roleHTTPTestRoleID, jsonBody(body))
		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.Empty(t, recorder.Body.String())

		require.Equal(t, roleHTTPTestRoleUUID, gotCommand.RoleID)
		require.Equal(t, "审计员", gotCommand.Name)
		require.Equal(t, "审计角色", gotCommand.Description)
		require.False(t, gotCommand.Active)
	})

	t.Run("validation failure is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPatch, "/api/v1/roles/"+roleHTTPTestRoleID, jsonBody(`{"description":"missing name","active":true}`))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("protected system role maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().UpdateRole(gomock.Any(), gomock.Any()).Return(roledomain.ErrSystemRoleProtected)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPatch, "/api/v1/roles/"+roleHTTPTestRoleID, jsonBody(`{"name":"管理员","active":false}`))
		expectRoleEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.SystemRoleProtected)
	})
}

func TestRoleControllerSetRoleStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		var gotCommand rolecommand.SetRoleActiveCommand
		commands.EXPECT().SetRoleActive(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.SetRoleActiveCommand) error {
			gotCommand = cmd
			return nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodPatch, "/api/v1/roles/"+roleHTTPTestRoleID+"/status", jsonBody(`{"active":false}`))
		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.Empty(t, recorder.Body.String())

		require.Equal(t, roleHTTPTestRoleUUID, gotCommand.RoleID)
		require.False(t, gotCommand.Active)
	})

	t.Run("invalid role id is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPatch, "/api/v1/roles/bad/status", jsonBody(`{"active":true}`))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("command service error maps to internal error", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().SetRoleActive(gomock.Any(), gomock.Any()).Return(errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodPatch, "/api/v1/roles/"+roleHTTPTestRoleID+"/status", jsonBody(`{"active":true}`))
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func newRoleHTTPTestHarness(t *testing.T) (*gin.Engine, *MockRoleCommandService, *MockRoleQueryService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	commands := NewMockRoleCommandService(ctrl)
	queries := NewMockRoleQueryService(ctrl)
	controller := NewRoleController(commands, queries, validator)
	engine := gin.New()
	RegisterRoleRoutes(engine.Group("/api/v1/roles"), controller)
	RegisterUserRoleRoutes(engine.Group("/api/v1/users"), controller)
	return engine, commands, queries
}

func performRoleHTTPRequest(t *testing.T, engine *gin.Engine, method string, target string, body *string) *httptest.ResponseRecorder {
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

func expectRoleEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, status int, success bool, code contracterrors.Code, message string) roleHTTPTestEnvelope {
	t.Helper()
	require.Equal(t, status, recorder.Code, "body=%s", recorder.Body.String())
	var envelope roleHTTPTestEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, success, envelope.Success)
	require.Equal(t, code, envelope.Code)
	if message != "" {
		require.Equal(t, message, envelope.Message)
	}
	return envelope
}

func decodeRoleHTTPData[T any](t *testing.T, envelope roleHTTPTestEnvelope) T {
	t.Helper()
	require.NotEmpty(t, envelope.Data)
	var data T
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	return data
}

func roleHTTPTestRole(roleID uuid.UUID, name string) roledomain.Role {
	return roledomain.Role{ID: 1, RoleID: roleID, Name: name, Description: name + "说明", Active: true, IsSystem: false, CreatedAt: 1780048800000, UpdatedAt: 1780052400000}
}

func roleHTTPTestPermission(permissionID uuid.UUID, name string) roleapplication.PermissionReference {
	return roleapplication.PermissionReference{ID: 1, PermissionID: permissionID, Name: name, Description: name + "说明", Module: "user", HTTPMethod: http.MethodGet, PathTemplate: "/api/v1/users", Active: true, IsSystem: true, CreatedAt: 1780048800000, UpdatedAt: 1780052400000}
}

func assertRoleHTTPResponse(t *testing.T, want roledomain.Role, got RoleResponse) {
	t.Helper()
	require.Equal(t, want.RoleID.String(), got.RoleID)
	require.Equal(t, want.Name, got.Name)
	require.Equal(t, want.Description, got.Description)
	require.Equal(t, want.Active, got.Active)
	require.Equal(t, want.IsSystem, got.System)
	require.Equal(t, want.CreatedAt, got.CreatedAt)
	require.Equal(t, want.UpdatedAt, got.UpdatedAt)
}

func assertPermissionHTTPResponse(t *testing.T, want roleapplication.PermissionReference, got PermissionResponse) {
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
