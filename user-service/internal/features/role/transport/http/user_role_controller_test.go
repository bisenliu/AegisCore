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
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/messages"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestRoleControllerListUserRoles(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		role := roleHTTPTestRole(roleHTTPTestRoleUUID, "管理员")
		var gotQuery rolequery.UserRolesQuery
		queries.EXPECT().ListUserRoles(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query rolequery.UserRolesQuery) (*rolequery.RolesResult, error) {
			gotQuery = query
			return &rolequery.RolesResult{Items: []roledomain.Role{role}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/users/"+roleHTTPTestUserID+"/roles", nil)
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]RoleResponse](t, envelope)

		require.Equal(t, roleHTTPTestUserUUID, gotQuery.UserID)
		require.Len(t, payload, 1)
		assertRoleHTTPResponse(t, role, payload[0])
	})

	t.Run("invalid user id is rejected before query service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/users/bad/roles", nil)
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("query service error maps to internal error", func(t *testing.T) {
		engine, _, queries := newRoleHTTPTestHarness(t)
		queries.EXPECT().ListUserRoles(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodGet, "/api/v1/users/"+roleHTTPTestUserID+"/roles", nil)
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestRoleControllerReplaceUserRoles(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		role := roleHTTPTestRole(roleHTTPTestRoleUUID, "管理员")
		var gotCommand rolecommand.ReplaceUserRolesCommand
		commands.EXPECT().ReplaceUserRoles(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.ReplaceUserRolesCommand) (*rolecommand.RolesResult, error) {
			gotCommand = cmd
			return &rolecommand.RolesResult{Items: []roledomain.Role{role}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodPut, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_ids":["`+roleHTTPTestRoleID+`"]}`))
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]RoleResponse](t, envelope)

		require.Equal(t, roleHTTPTestUserUUID, gotCommand.UserID)
		require.Equal(t, []uuid.UUID{roleHTTPTestRoleUUID}, gotCommand.RoleIDs)
		require.Len(t, payload, 1)
		assertRoleHTTPResponse(t, role, payload[0])
	})

	t.Run("invalid role id collection is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPut, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_ids":["bad"]}`))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("missing role maps to not found envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().ReplaceUserRoles(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRoleNotFound)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPut, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_ids":["`+roleHTTPTestRoleID+`"]}`))
		expectRoleEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.RoleNotFound)
	})

	t.Run("inactive role maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().ReplaceUserRoles(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRoleInactive)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPut, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_ids":["`+roleHTTPTestRoleID+`"]}`))
		expectRoleEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.RoleInactive)
	})
}

func TestRoleControllerAddUserRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		role := roleHTTPTestRole(roleHTTPTestRoleUUID, "管理员")
		var gotCommand rolecommand.UserRoleCommand
		commands.EXPECT().AddUserRole(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.UserRoleCommand) (*rolecommand.RolesResult, error) {
			gotCommand = cmd
			return &rolecommand.RolesResult{Items: []roledomain.Role{role}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_id":"`+roleHTTPTestRoleID+`"}`))
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]RoleResponse](t, envelope)

		require.Equal(t, roleHTTPTestUserUUID, gotCommand.UserID)
		require.Equal(t, roleHTTPTestRoleUUID, gotCommand.RoleID)
		require.Len(t, payload, 1)
		assertRoleHTTPResponse(t, role, payload[0])
	})

	t.Run("invalid user id is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/users/bad/roles", jsonBody(`{"role_id":"`+roleHTTPTestRoleID+`"}`))
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("duplicate binding maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().AddUserRole(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrUserRoleAlreadyExists)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_id":"`+roleHTTPTestRoleID+`"}`))
		expectRoleEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.UserRoleAlreadyExists)
	})

	t.Run("inactive role maps to conflict envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().AddUserRole(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrRoleInactive)

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_id":"`+roleHTTPTestRoleID+`"}`))
		expectRoleEnvelope(t, recorder, http.StatusConflict, false, contracterrors.CodeConflict, messages.RoleInactive)
	})

	t.Run("command service error maps to internal error", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().AddUserRole(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))

		recorder := performRoleHTTPRequest(t, engine, http.MethodPost, "/api/v1/users/"+roleHTTPTestUserID+"/roles", jsonBody(`{"role_id":"`+roleHTTPTestRoleID+`"}`))
		expectRoleEnvelope(t, recorder, http.StatusInternalServerError, false, contracterrors.CodeInternalError, contractresponse.MessageInternalError)
	})
}

func TestRoleControllerRemoveUserRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		role := roleHTTPTestRole(roleHTTPTestSecondRoleUUID, "审计员")
		var gotCommand rolecommand.UserRoleCommand
		commands.EXPECT().RemoveUserRole(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd rolecommand.UserRoleCommand) (*rolecommand.RolesResult, error) {
			gotCommand = cmd
			return &rolecommand.RolesResult{Items: []roledomain.Role{role}}, nil
		})

		recorder := performRoleHTTPRequest(t, engine, http.MethodDelete, "/api/v1/users/"+roleHTTPTestUserID+"/roles/"+roleHTTPTestRoleID, nil)
		envelope := expectRoleEnvelope(t, recorder, http.StatusOK, true, contracterrors.CodeOK, contractresponse.MessageOK)
		payload := decodeRoleHTTPData[[]RoleResponse](t, envelope)

		require.Equal(t, roleHTTPTestUserUUID, gotCommand.UserID)
		require.Equal(t, roleHTTPTestRoleUUID, gotCommand.RoleID)
		require.Len(t, payload, 1)
		assertRoleHTTPResponse(t, role, payload[0])
	})

	t.Run("invalid role id is rejected before command service", func(t *testing.T) {
		engine, _, _ := newRoleHTTPTestHarness(t)
		recorder := performRoleHTTPRequest(t, engine, http.MethodDelete, "/api/v1/users/"+roleHTTPTestUserID+"/roles/bad", nil)
		expectRoleEnvelope(t, recorder, http.StatusBadRequest, false, contracterrors.CodeValidationFailed, validation.ErrValidationFailed)
	})

	t.Run("binding not found maps to not found envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().RemoveUserRole(gomock.Any(), gomock.Any()).Return(nil, roledomain.ErrUserRoleNotFound)

		recorder := performRoleHTTPRequest(t, engine, http.MethodDelete, "/api/v1/users/"+roleHTTPTestUserID+"/roles/"+roleHTTPTestRoleID, nil)
		expectRoleEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.UserRoleNotFound)
	})

	t.Run("missing user maps to not found envelope", func(t *testing.T) {
		engine, commands, _ := newRoleHTTPTestHarness(t)
		commands.EXPECT().RemoveUserRole(gomock.Any(), gomock.Any()).Return(nil, errors.Join(errors.New("lookup user"), identity.ErrUserNotFound))

		recorder := performRoleHTTPRequest(t, engine, http.MethodDelete, "/api/v1/users/"+roleHTTPTestUserID+"/roles/"+roleHTTPTestRoleID, nil)
		expectRoleEnvelope(t, recorder, http.StatusNotFound, false, contracterrors.CodeNotFound, messages.UserNotFound)
	})
}
