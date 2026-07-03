package userhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/validation"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

const controllerTestUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

var controllerTestUUID = uuid.MustParse(controllerTestUserID)

func TestUserControllerGetByUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid ID", func(t *testing.T) {
		createdAt := int64(1780048800000)
		updatedAt := int64(1780052400000)
		service := NewMockUserQueryService(gomock.NewController(t))
		var gotID uuid.UUID
		service.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query userquery.GetUserByIDQuery) (*userquery.GetUserResult, error) {
			gotID = query.UserID
			return &userquery.GetUserResult{User: userdomain.User{UserID: controllerTestUUID, Nickname: "Aegis", Username: "aegis", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: updatedAt}}, nil
		})

		status, envelope := executeGetByUserID(t, service, controllerTestUserID)

		require.Equal(t, http.StatusOK, status)
		require.Equal(t, controllerTestUserID, gotID.String())
		require.True(t, envelope.Success)
		require.Equal(t, contracterrors.CodeOK, envelope.Code)
		require.Equal(t, "ok", envelope.Message)
		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)
		require.Equal(t, controllerTestUserID, data["user_id"])
		require.Equal(t, "Aegis", data["nickname"])
		require.Equal(t, "aegis", data["username"])
		require.Equal(t, float64(identity.UserStatusNormal), data["status"])
		require.Equal(t, float64(createdAt), data["created_at"])
		require.Equal(t, float64(updatedAt), data["updated_at"])
		require.NotContains(t, data, "id")
		require.NotContains(t, data, "e"+"mail")
	})

	t.Run("invalid UUID", func(t *testing.T) {
		status, envelope := executeGetByUserID(t, NewMockUserQueryService(gomock.NewController(t)), "abc")
		assertInvalidUserID(t, status, envelope, validation.ErrValidationFailed)
	})

	t.Run("not found", func(t *testing.T) {
		service := NewMockUserQueryService(gomock.NewController(t))
		service.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(nil, identity.ErrUserNotFound)
		status, envelope := executeGetByUserID(t, service, controllerTestUserID)
		require.Equal(t, http.StatusNotFound, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeNotFound, envelope.Code)
		require.Equal(t, messages.UserNotFound, envelope.Message)
	})

	t.Run("service error", func(t *testing.T) {
		service := NewMockUserQueryService(gomock.NewController(t))
		service.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))
		status, envelope := executeGetByUserID(t, service, controllerTestUserID)
		require.Equal(t, http.StatusInternalServerError, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeInternalError, envelope.Code)
		require.Equal(t, "internal server error", envelope.Message)
	})
}

func TestUserControllerCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	createdAt := int64(1780048800000)
	createdUser := &usercommand.CreateUserResult{User: userdomain.User{UserID: controllerTestUUID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}

	t.Run("valid body", func(t *testing.T) {
		service := NewMockCreateUserService(gomock.NewController(t))
		var gotCreate usercommand.CreateUserCommand
		service.EXPECT().CreateUser(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, cmd usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
			gotCreate = cmd
			return createdUser, nil
		})

		status, envelope := executeCreate(t, service, `{"nickname":"Alice","username":"ALICE","password":"secret"}`)

		require.Equal(t, http.StatusCreated, status)
		require.Equal(t, "Alice", gotCreate.Nickname)
		require.Equal(t, "alice", gotCreate.Username)
		require.Equal(t, "secret", gotCreate.Password)
		require.NotNil(t, gotCreate.Status)
		require.Equal(t, identity.UserStatusNormal, *gotCreate.Status)
		require.True(t, envelope.Success)
		require.Equal(t, contracterrors.CodeOK, envelope.Code)
		require.Equal(t, "created", envelope.Message)
		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)
		require.Equal(t, controllerTestUserID, data["user_id"])
		require.Equal(t, "Alice", data["nickname"])
		require.Equal(t, "alice", data["username"])
		require.Equal(t, float64(identity.UserStatusNormal), data["status"])
		require.Equal(t, float64(createdAt), data["created_at"])
		require.NotContains(t, data, "id")
		require.NotContains(t, data, "e"+"mail")
	})

	t.Run("empty body", func(t *testing.T) {
		status, envelope := executeCreate(t, NewMockCreateUserService(gomock.NewController(t)), "")
		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeBadRequest, envelope.Code)
		require.Equal(t, validation.ErrEmptyRequestBody, envelope.Message)
	})

	t.Run("validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, NewMockCreateUserService(gomock.NewController(t)), `{"nickname":"Alice","password":"secret"}`)
		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeValidationFailed, envelope.Code)
		require.Equal(t, validation.ErrValidationFailed, envelope.Message)
		assertFieldError(t, envelope, "username", "用户名", "required", "用户名为必填字段")
	})

	t.Run("invalid status validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, NewMockCreateUserService(gomock.NewController(t)), `{"nickname":"Alice","username":"alice","password":"secret","status":999}`)
		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeValidationFailed, envelope.Code)
		require.Equal(t, validation.ErrValidationFailed, envelope.Message)
		assertFieldError(t, envelope, "status", "用户状态", "enum", "用户状态取值不合法，允许值为：100、200、300")
	})

	t.Run("missing password validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, NewMockCreateUserService(gomock.NewController(t)), `{"nickname":"Alice","username":"alice"}`)
		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeValidationFailed, envelope.Code)
		require.Equal(t, validation.ErrValidationFailed, envelope.Message)
		assertFieldError(t, envelope, "password", "密码", "required", "密码为必填字段")
	})

	t.Run("user already exists", func(t *testing.T) {
		service := NewMockCreateUserService(gomock.NewController(t))
		service.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil, identity.ErrUserAlreadyExists)
		status, envelope := executeCreate(t, service, `{"nickname":"Alice","username":"alice","password":"secret"}`)
		require.Equal(t, http.StatusConflict, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeConflict, envelope.Code)
		require.Equal(t, messages.UserAlreadyExists, envelope.Message)
	})

	t.Run("service error", func(t *testing.T) {
		service := NewMockCreateUserService(gomock.NewController(t))
		service.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))
		status, envelope := executeCreate(t, service, `{"nickname":"Alice","username":"alice","password":"secret"}`)
		require.Equal(t, http.StatusInternalServerError, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeInternalError, envelope.Code)
		require.Equal(t, "internal server error", envelope.Message)
	})
}

func TestUserControllerList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	createdAt := int64(1780048800000)
	listResponse := &userquery.ListUsersResult{Items: []userdomain.User{{UserID: controllerTestUUID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}, PageSize: 20, NextCursor: controllerTestUserID, HasNext: true}

	t.Run("default pagination", func(t *testing.T) {
		service := NewMockUserQueryService(gomock.NewController(t))
		var gotList userquery.ListUsersQuery
		service.EXPECT().ListUsers(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query userquery.ListUsersQuery) (*userquery.ListUsersResult, error) {
			gotList = query
			return &userquery.ListUsersResult{Items: []userdomain.User{}, PageSize: 10}, nil
		})

		status, envelope := executeList(t, service, "/api/v1/users")

		require.Equal(t, http.StatusOK, status)
		require.Nil(t, gotList.Cursor)
		require.Equal(t, 10, gotList.PageSize)
		require.Equal(t, 10, gotList.Limit)
		assertPaginatedEnvelope(t, envelope, 10, "", false, 0)
	})

	t.Run("explicit query", func(t *testing.T) {
		service := NewMockUserQueryService(gomock.NewController(t))
		var gotList userquery.ListUsersQuery
		service.EXPECT().ListUsers(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query userquery.ListUsersQuery) (*userquery.ListUsersResult, error) {
			gotList = query
			return listResponse, nil
		})

		status, envelope := executeList(t, service, "/api/v1/users?cursor=018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4d&page_size=20&nickname=%20Ali%20&username=%20alice%20&status=100")

		require.Equal(t, http.StatusOK, status)
		require.NotNil(t, gotList.Cursor)
		require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4d", gotList.Cursor.String())
		require.Equal(t, 20, gotList.PageSize)
		require.Equal(t, 20, gotList.Limit)
		require.Equal(t, "Ali", gotList.Nickname)
		require.Equal(t, "alice", gotList.Username)
		require.NotNil(t, gotList.Status)
		require.Equal(t, identity.UserStatusNormal, *gotList.Status)
		assertPaginatedEnvelope(t, envelope, 20, controllerTestUserID, true, 1)
	})

	t.Run("page size capped", func(t *testing.T) {
		service := NewMockUserQueryService(gomock.NewController(t))
		var gotList userquery.ListUsersQuery
		service.EXPECT().ListUsers(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, query userquery.ListUsersQuery) (*userquery.ListUsersResult, error) {
			gotList = query
			return &userquery.ListUsersResult{Items: []userdomain.User{}, PageSize: 100}, nil
		})

		status, envelope := executeList(t, service, "/api/v1/users?page_size=101")

		require.Equal(t, http.StatusOK, status)
		require.Equal(t, 100, gotList.PageSize)
		require.Equal(t, 100, gotList.Limit)
		assertPaginatedEnvelope(t, envelope, 100, "", false, 0)
	})

	t.Run("invalid cursor", func(t *testing.T) {
		service := NewMockUserQueryService(gomock.NewController(t))

		status, envelope := executeList(t, service, "/api/v1/users?cursor=abc")

		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeBadRequest, envelope.Code)
		require.Equal(t, messages.InvalidUserID, envelope.Message)
	})

	t.Run("invalid status", func(t *testing.T) {
		status, envelope := executeList(t, NewMockUserQueryService(gomock.NewController(t)), "/api/v1/users?status=999")
		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeValidationFailed, envelope.Code)
		require.Equal(t, validation.ErrValidationFailed, envelope.Message)
		assertFieldError(t, envelope, "status", "用户状态", "enum", "用户状态取值不合法，允许值为：100、200、300")
	})

	t.Run("service error", func(t *testing.T) {
		service := NewMockUserQueryService(gomock.NewController(t))
		service.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Return(nil, errors.New("database down"))
		status, envelope := executeList(t, service, "/api/v1/users")
		require.Equal(t, http.StatusInternalServerError, status)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeInternalError, envelope.Code)
		require.Equal(t, response.MessageInternalError, envelope.Message)
	})
}

func executeCreate(t *testing.T, commands usercommand.CreateUserService, body string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	ctl := NewUserController(commands, NewMockUserQueryService(gomock.NewController(t)), validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = int64(len(body))
	ctx.Request = request

	ctl.CreateUser(ctx)

	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return recorder.Code, envelope
}

func executeGetByUserID(t *testing.T, queries userquery.UserQueryService, id string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	ctl := NewUserController(NewMockCreateUserService(gomock.NewController(t)), queries, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+id, nil)
	ctx.Params = gin.Params{{Key: "user_id", Value: id}}

	ctl.GetByUserID(ctx)

	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return recorder.Code, envelope
}

func executeList(t *testing.T, queries userquery.UserQueryService, path string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	ctl := NewUserController(NewMockCreateUserService(gomock.NewController(t)), queries, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)

	ctl.ListUsers(ctx)

	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return recorder.Code, envelope
}

func assertPaginatedEnvelope(t *testing.T, envelope response.Envelope, pageSize int, nextCursor string, hasNext bool, itemCount int) {
	t.Helper()
	require.True(t, envelope.Success)
	require.Equal(t, contracterrors.CodeOK, envelope.Code)
	require.Equal(t, response.MessageOK, envelope.Message)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	items, ok := data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, itemCount)
	if itemCount > 0 {
		item, ok := items[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, controllerTestUserID, item["user_id"])
		require.Equal(t, "Alice", item["nickname"])
		require.Equal(t, "alice", item["username"])
		require.Equal(t, float64(identity.UserStatusNormal), item["status"])
		require.NotContains(t, item, "id")
		require.NotContains(t, item, "e"+"mail")
	}
	pagination, ok := data["pagination"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(pageSize), pagination["page_size"])
	require.Equal(t, hasNext, pagination["has_next"])
	if nextCursor == "" {
		require.NotContains(t, pagination, "next_cursor")
	} else if pagination["next_cursor"] != nextCursor {
		require.Equal(t, nextCursor, pagination["next_cursor"])
	}
	for _, removed := range []string{"page", "offset", "total", "total_pages"} {
		require.NotContains(t, pagination, removed)
	}
}

func assertInvalidUserID(t *testing.T, status int, envelope response.Envelope, message string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, status)
	require.False(t, envelope.Success)
	require.Equal(t, contracterrors.CodeValidationFailed, envelope.Code)
	require.Equal(t, message, envelope.Message)
	require.NotEqual(t, "invalid user id", envelope.Message)
}

func assertFieldError(t *testing.T, envelope response.Envelope, field, label, rule, message string) {
	t.Helper()
	errors, ok := envelope.Errors.([]any)
	require.True(t, ok)
	require.Len(t, errors, 1)
	fieldError, ok := errors[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, field, fieldError["field"])
	require.Equal(t, label, fieldError["label"])
	require.Equal(t, rule, fieldError["rule"])
	require.Equal(t, message, fieldError["message"])
}
