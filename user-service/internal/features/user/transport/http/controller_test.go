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
		service := &stubUserQueries{response: &userquery.GetUserResult{User: userdomain.User{UserID: controllerTestUUID, Nickname: "Aegis", Username: "aegis", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: updatedAt}}}

		status, envelope := executeGetByUserID(t, service, controllerTestUserID)

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotID.String() != controllerTestUserID {
			t.Fatalf("gotID = %q", service.gotID)
		}
		if !envelope.Success || envelope.Code != contracterrors.CodeOK || envelope.Message != "ok" {
			t.Fatalf("envelope = %#v", envelope)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data = %T, want map", envelope.Data)
		}
		if data["user_id"] != controllerTestUserID || data["nickname"] != "Aegis" || data["username"] != "aegis" || data["status"] != float64(identity.UserStatusNormal) || data["created_at"] != float64(createdAt) || data["updated_at"] != float64(updatedAt) {
			t.Fatalf("data = %#v", data)
		}
		if _, ok := data["id"]; ok {
			t.Fatalf("data = %#v", data)
		}
		if _, ok := data["e"+"mail"]; ok {
			t.Fatalf("data = %#v", data)
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		status, envelope := executeGetByUserID(t, &stubUserQueries{}, "abc")
		assertInvalidUserID(t, status, envelope, validation.ErrValidationFailed)
	})

	t.Run("not found", func(t *testing.T) {
		status, envelope := executeGetByUserID(t, &stubUserQueries{err: identity.ErrUserNotFound}, controllerTestUserID)
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", status, http.StatusNotFound)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeNotFound || envelope.Message != messages.UserNotFound {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("service error", func(t *testing.T) {
		status, envelope := executeGetByUserID(t, &stubUserQueries{err: errors.New("database down")}, controllerTestUserID)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeInternalError || envelope.Message != "internal server error" {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

func TestUserControllerCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	createdAt := int64(1780048800000)
	createdUser := &usercommand.CreateUserResult{User: userdomain.User{UserID: controllerTestUUID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}

	t.Run("valid body", func(t *testing.T) {
		service := &stubUserCommands{createResponse: createdUser}

		status, envelope := executeCreate(t, service, `{"nickname":"Alice","username":"ALICE","password":"secret"}`)

		if status != http.StatusCreated {
			t.Fatalf("status = %d, want %d", status, http.StatusCreated)
		}
		if service.gotCreate.Nickname != "Alice" || service.gotCreate.Username != "alice" || service.gotCreate.Password != "secret" || service.gotCreate.Status == nil || *service.gotCreate.Status != identity.UserStatusNormal {
			t.Fatalf("gotCreate = %#v", service.gotCreate)
		}
		if !envelope.Success || envelope.Code != contracterrors.CodeOK || envelope.Message != "created" {
			t.Fatalf("envelope = %#v", envelope)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok || data["user_id"] != controllerTestUserID || data["nickname"] != "Alice" || data["username"] != "alice" || data["status"] != float64(identity.UserStatusNormal) || data["created_at"] != float64(createdAt) {
			t.Fatalf("data = %#v", envelope.Data)
		}
		if _, ok := data["id"]; ok {
			t.Fatalf("data = %#v", envelope.Data)
		}
		if _, ok := data["e"+"mail"]; ok {
			t.Fatalf("data = %#v", envelope.Data)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserCommands{}, "")
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeBadRequest || envelope.Message != validation.ErrEmptyRequestBody {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserCommands{}, `{"nickname":"Alice","password":"secret"}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "username", "用户名", "required", "用户名为必填字段")
	})

	t.Run("invalid status validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserCommands{}, `{"nickname":"Alice","username":"alice","password":"secret","status":999}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "status", "用户状态", "enum", "用户状态取值不合法，允许值为：100、200、300")
	})

	t.Run("missing password validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserCommands{}, `{"nickname":"Alice","username":"alice"}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "password", "密码", "required", "密码为必填字段")
	})

	t.Run("user already exists", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserCommands{createErr: identity.ErrUserAlreadyExists}, `{"nickname":"Alice","username":"alice","password":"secret"}`)
		if status != http.StatusConflict {
			t.Fatalf("status = %d, want %d", status, http.StatusConflict)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeConflict || envelope.Message != messages.UserAlreadyExists {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("service error", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserCommands{createErr: errors.New("database down")}, `{"nickname":"Alice","username":"alice","password":"secret"}`)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeInternalError || envelope.Message != "internal server error" {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

func TestUserControllerList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	createdAt := int64(1780048800000)
	listResponse := &userquery.ListUsersResult{Items: []userdomain.User{{UserID: controllerTestUUID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}, PageSize: 20, NextCursor: controllerTestUserID, HasNext: true}

	t.Run("default pagination", func(t *testing.T) {
		service := &stubUserQueries{listResponse: &userquery.ListUsersResult{Items: []userdomain.User{}, PageSize: 10}}

		status, envelope := executeList(t, service, "/api/v1/users")

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotList.Cursor != nil || service.gotList.PageSize != 10 || service.gotList.Limit != 10 {
			t.Fatalf("gotList = %#v", service.gotList)
		}
		assertPaginatedEnvelope(t, envelope, 10, "", false, 0)
	})

	t.Run("explicit query", func(t *testing.T) {
		service := &stubUserQueries{listResponse: listResponse}

		status, envelope := executeList(t, service, "/api/v1/users?cursor=018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4d&page_size=20&nickname=%20Ali%20&username=%20alice%20&status=100")

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotList.Cursor == nil || service.gotList.Cursor.String() != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4d" || service.gotList.PageSize != 20 || service.gotList.Limit != 20 || service.gotList.Nickname != "Ali" || service.gotList.Username != "alice" {
			t.Fatalf("gotList = %#v", service.gotList)
		}
		if service.gotList.Status == nil || *service.gotList.Status != identity.UserStatusNormal {
			t.Fatalf("status = %#v", service.gotList.Status)
		}
		assertPaginatedEnvelope(t, envelope, 20, controllerTestUserID, true, 1)
	})

	t.Run("page size capped", func(t *testing.T) {
		service := &stubUserQueries{listResponse: &userquery.ListUsersResult{Items: []userdomain.User{}, PageSize: 100}}

		status, envelope := executeList(t, service, "/api/v1/users?page_size=101")

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotList.PageSize != 100 || service.gotList.Limit != 100 {
			t.Fatalf("gotList = %#v", service.gotList)
		}
		assertPaginatedEnvelope(t, envelope, 100, "", false, 0)
	})

	t.Run("invalid cursor", func(t *testing.T) {
		service := &stubUserQueries{}

		status, envelope := executeList(t, service, "/api/v1/users?cursor=abc")

		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if service.gotList.Limit != 0 {
			t.Fatalf("service should not be called, gotList = %#v", service.gotList)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeBadRequest || envelope.Message != messages.InvalidUserID {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		status, envelope := executeList(t, &stubUserQueries{}, "/api/v1/users?status=999")
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "status", "用户状态", "enum", "用户状态取值不合法，允许值为：100、200、300")
	})

	t.Run("service error", func(t *testing.T) {
		status, envelope := executeList(t, &stubUserQueries{listErr: errors.New("database down")}, "/api/v1/users")
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
		}
		if envelope.Success || envelope.Code != contracterrors.CodeInternalError || envelope.Message != response.MessageInternalError {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

type stubUserCommands struct {
	createResponse *usercommand.CreateUserResult
	createErr      error
	gotCreate      usercommand.CreateUserCommand
}

type stubUserQueries struct {
	response     *userquery.GetUserResult
	err          error
	gotID        uuid.UUID
	listResponse *userquery.ListUsersResult
	listErr      error
	gotList      userquery.ListUsersQuery
}

func (s *stubUserCommands) CreateUser(_ context.Context, req usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
	s.gotCreate = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	return s.createResponse, nil
}

func (s *stubUserQueries) GetUserByID(_ context.Context, req userquery.GetUserByIDQuery) (*userquery.GetUserResult, error) {
	s.gotID = req.UserID
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func (s *stubUserQueries) ListUsers(_ context.Context, req userquery.ListUsersQuery) (*userquery.ListUsersResult, error) {
	s.gotList = req
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listResponse, nil
}

func executeCreate(t *testing.T, commands *stubUserCommands, body string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	ctl := NewUserController(commands, &stubUserQueries{}, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = int64(len(body))
	ctx.Request = request

	ctl.CreateUser(ctx)

	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}

func executeGetByUserID(t *testing.T, queries *stubUserQueries, id string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	ctl := NewUserController(&stubUserCommands{}, queries, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+id, nil)
	ctx.Params = gin.Params{{Key: "user_id", Value: id}}

	ctl.GetByUserID(ctx)

	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}

func executeList(t *testing.T, queries *stubUserQueries, path string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	ctl := NewUserController(&stubUserCommands{}, queries, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)

	ctl.ListUsers(ctx)

	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}

func assertPaginatedEnvelope(t *testing.T, envelope response.Envelope, pageSize int, nextCursor string, hasNext bool, itemCount int) {
	t.Helper()
	if !envelope.Success || envelope.Code != contracterrors.CodeOK || envelope.Message != response.MessageOK {
		t.Fatalf("envelope = %#v", envelope)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want map", envelope.Data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != itemCount {
		t.Fatalf("items = %#v", data["items"])
	}
	if itemCount > 0 {
		item, ok := items[0].(map[string]any)
		if !ok || item["user_id"] != controllerTestUserID || item["nickname"] != "Alice" || item["username"] != "alice" || item["status"] != float64(identity.UserStatusNormal) {
			t.Fatalf("item = %#v", items[0])
		}
		if _, ok := item["id"]; ok {
			t.Fatalf("item = %#v", item)
		}
		if _, ok := item["e"+"mail"]; ok {
			t.Fatalf("item = %#v", item)
		}
	}
	pagination, ok := data["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("pagination = %#v", data["pagination"])
	}
	if pagination["page_size"] != float64(pageSize) || pagination["has_next"] != hasNext {
		t.Fatalf("pagination = %#v", pagination)
	}
	if nextCursor == "" {
		if _, ok := pagination["next_cursor"]; ok {
			t.Fatalf("pagination = %#v, want next_cursor omitted", pagination)
		}
	} else if pagination["next_cursor"] != nextCursor {
		t.Fatalf("pagination = %#v", pagination)
	}
	for _, removed := range []string{"page", "offset", "total", "total_pages"} {
		if _, ok := pagination[removed]; ok {
			t.Fatalf("pagination contains removed field %q: %#v", removed, pagination)
		}
	}
}

func assertInvalidUserID(t *testing.T, status int, envelope response.Envelope, message string) {
	t.Helper()
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeValidationFailed || envelope.Message != message || envelope.Message == "invalid user id" {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func assertFieldError(t *testing.T, envelope response.Envelope, field, label, rule, message string) {
	t.Helper()
	errors, ok := envelope.Errors.([]any)
	if !ok || len(errors) != 1 {
		t.Fatalf("errors = %#v", envelope.Errors)
	}
	fieldError, ok := errors[0].(map[string]any)
	if !ok || fieldError["field"] != field || fieldError["label"] != label || fieldError["rule"] != rule || fieldError["message"] != message {
		t.Fatalf("field error = %#v", errors[0])
	}
}
