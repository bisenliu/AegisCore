package rolehttp

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/validation"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestRoleControllerListRolePermissions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		permission := roleHTTPTestPermission(roleHTTPTestPermissionUUID, "查询用户")
		var gotQuery rolequery.RolePermissionsQuery
		queries.EXPECT().ListRolePermissions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query rolequery.RolePermissionsQuery) (*rolequery.PermissionsResult, error) {
			gotQuery = query
			return &rolequery.PermissionsResult{Items: []roleapplication.PermissionReference{permission}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", nil)
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]PermissionResponse](t, envelope)

		require.Equal(t, roleHTTPTestRoleUUID, gotQuery.RoleID)
		require.Len(t, payload, 1)
		assertPermissionHTTPResponse(t, permission, payload[0])
	})

	t.Run("invalid role id is rejected before query service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles/bad/permissions", nil)
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		queries.EXPECT().ListRolePermissions(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", nil)
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestRoleControllerReplaceRolePermissions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		permission := roleHTTPTestPermission(roleHTTPTestPermissionUUID, "查询用户")
		var gotCommand rolecommand.ReplaceRolePermissionsCommand
		commands.EXPECT().ReplaceRolePermissions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.ReplaceRolePermissionsCommand) (*rolecommand.PermissionsResult, error) {
			gotCommand = cmd
			return &rolecommand.PermissionsResult{Items: []roleapplication.PermissionReference{permission}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodPut, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", jsonBody(`{"permission_ids":["`+roleHTTPTestPermissionID+`"]}`))
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]PermissionResponse](t, envelope)

		require.Equal(t, roleHTTPTestRoleUUID, gotCommand.RoleID)
		require.Equal(t, []uuid.UUID{roleHTTPTestPermissionUUID}, gotCommand.PermissionIDs)
		require.Len(t, payload, 1)
		assertPermissionHTTPResponse(t, permission, payload[0])
	})

	t.Run("invalid permission id collection is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPut, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", jsonBody(`{"permission_ids":["bad"]}`))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("missing permission maps to not found envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().ReplaceRolePermissions(gomock.Any(), gomock.Any()).Return(nil, errors.Join(errors.New("permission lookup failed"), permissiondomain.ErrPermissionNotFound))

		recorder := performRoleHTTPRequest(t, engine, http.MethodPut, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", jsonBody(`{"permission_ids":["`+roleHTTPTestPermissionID+`"]}`))
		expectRoleEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.PermissionNotFound)
	})
}

func TestRoleControllerAddRolePermission(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		permission := roleHTTPTestPermission(roleHTTPTestPermissionUUID, "查询用户")
		var gotCommand rolecommand.RolePermissionCommand
		commands.EXPECT().AddRolePermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.RolePermissionCommand) (*rolecommand.PermissionsResult, error) {
			gotCommand = cmd
			return &rolecommand.PermissionsResult{Items: []roleapplication.PermissionReference{permission}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", jsonBody(`{"permission_id":"`+roleHTTPTestPermissionID+`"}`))
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]PermissionResponse](t, envelope)

		require.Equal(t, roleHTTPTestRoleUUID, gotCommand.RoleID)
		require.Equal(t, roleHTTPTestPermissionUUID, gotCommand.PermissionID)
		require.Len(t, payload, 1)
		assertPermissionHTTPResponse(t, permission, payload[0])
	})

	t.Run("invalid role id is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles/bad/permissions", jsonBody(`{"permission_id":"`+roleHTTPTestPermissionID+`"}`))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("conflict maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().AddRolePermission(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRolePermissionAlreadyExists)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", jsonBody(`{"permission_id":"`+roleHTTPTestPermissionID+`"}`))
		expectRoleEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.RolePermissionAlreadyExists)
	})

	t.Run("command service error maps to internal error", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().AddRolePermission(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions", jsonBody(`{"permission_id":"`+roleHTTPTestPermissionID+`"}`))
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestRoleControllerRemoveRolePermission(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		permission := roleHTTPTestPermission(roleHTTPTestPermissionUUID, "查询用户")
		var gotCommand rolecommand.RolePermissionCommand
		commands.EXPECT().RemoveRolePermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.RolePermissionCommand) (*rolecommand.PermissionsResult, error) {
			gotCommand = cmd
			return &rolecommand.PermissionsResult{Items: []roleapplication.PermissionReference{permission}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodDelete, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions/"+roleHTTPTestPermissionID, nil)
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]PermissionResponse](t, envelope)

		require.Equal(t, roleHTTPTestRoleUUID, gotCommand.RoleID)
		require.Equal(t, roleHTTPTestPermissionUUID, gotCommand.PermissionID)
		require.Len(t, payload, 1)
		assertPermissionHTTPResponse(t, permission, payload[0])
	})

	t.Run("invalid permission id is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodDelete, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions/bad", nil)
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("not found maps to not found envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().RemoveRolePermission(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRolePermissionNotFound)

		recorder := performRoleHTTPRequest(t, engine, http.MethodDelete, "/api/v1/roles/"+roleHTTPTestRoleID+"/permissions/"+roleHTTPTestPermissionID, nil)
		expectRoleEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.RolePermissionNotFound)
	})
}
