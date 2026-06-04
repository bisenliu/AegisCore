package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/errmsg"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const controllerTestUserID = "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"

func TestUserControllerGetByID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid ID", func(t *testing.T) {
		createdAt := int64(1780048800000)
		updatedAt := int64(1780052400000)
		service := &stubUserService{response: &dto.UserResponse{UserID: controllerTestUserID, Nickname: "Aegis", Username: "aegis", Status: domain.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: updatedAt}}

		status, envelope := executeGetByID(t, service, controllerTestUserID)

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotID.String() != controllerTestUserID {
			t.Fatalf("gotID = %q", service.gotID)
		}
		if !envelope.Success || envelope.Code != response.CodeOK || envelope.Message != "ok" {
			t.Fatalf("envelope = %#v", envelope)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data = %T, want map", envelope.Data)
		}
		if data["user_id"] != controllerTestUserID || data["nickname"] != "Aegis" || data["username"] != "aegis" || data["status"] != float64(domain.UserStatusNormal) || data["created_at"] != float64(createdAt) || data["updated_at"] != float64(updatedAt) {
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
		status, envelope := executeGetByID(t, &stubUserService{}, "abc")
		assertInvalidUserID(t, status, envelope, validation.ErrValidationFailed)
	})

	t.Run("not found", func(t *testing.T) {
		status, envelope := executeGetByID(t, &stubUserService{err: response.NotFoundError(errmsg.MsgUserNotFound)}, controllerTestUserID)
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", status, http.StatusNotFound)
		}
		if envelope.Success || envelope.Code != response.CodeNotFound || envelope.Message != errmsg.MsgUserNotFound {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("service error", func(t *testing.T) {
		status, envelope := executeGetByID(t, &stubUserService{err: errors.New("database down")}, controllerTestUserID)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
		}
		if envelope.Success || envelope.Code != response.CodeInternalError || envelope.Message != "internal server error" {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

func TestUserControllerCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	createdAt := int64(1780048800000)
	createdUser := &dto.UserResponse{UserID: controllerTestUserID, Nickname: "Alice", Username: "alice", Status: domain.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}

	t.Run("valid body", func(t *testing.T) {
		service := &stubUserService{createResponse: createdUser}

		status, envelope := executeCreate(t, service, `{"nickname":"Alice","username":"alice","password":"secret"}`)

		if status != http.StatusCreated {
			t.Fatalf("status = %d, want %d", status, http.StatusCreated)
		}
		if service.gotCreate.Nickname != "Alice" || service.gotCreate.Username != "alice" || service.gotCreate.Password != "secret" || service.gotCreate.Status == nil || *service.gotCreate.Status != domain.UserStatusNormal {
			t.Fatalf("gotCreate = %#v", service.gotCreate)
		}
		if !envelope.Success || envelope.Code != response.CodeOK || envelope.Message != "created" {
			t.Fatalf("envelope = %#v", envelope)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok || data["user_id"] != controllerTestUserID || data["nickname"] != "Alice" || data["username"] != "alice" || data["status"] != float64(domain.UserStatusNormal) || data["created_at"] != float64(createdAt) {
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
		status, envelope := executeCreate(t, &stubUserService{}, "")
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != response.CodeBadRequest || envelope.Message != validation.ErrEmptyRequestBody {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserService{}, `{"nickname":"Alice","password":"secret"}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != response.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "username", "用户名", "required", "用户名为必填字段")
	})

	t.Run("invalid status validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserService{}, `{"nickname":"Alice","username":"alice","password":"secret","status":999}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != response.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "status", "用户状态", "enum", "用户状态取值不合法，允许值为：100、200、300")
	})

	t.Run("missing password validation failed", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserService{}, `{"nickname":"Alice","username":"alice"}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != response.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "password", "密码", "required", "密码为必填字段")
	})

	t.Run("user already exists", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserService{createErr: response.ConflictError(errmsg.MsgUserAlreadyExists)}, `{"nickname":"Alice","username":"alice","password":"secret"}`)
		if status != http.StatusConflict {
			t.Fatalf("status = %d, want %d", status, http.StatusConflict)
		}
		if envelope.Success || envelope.Code != response.CodeConflict || envelope.Message != errmsg.MsgUserAlreadyExists {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("service error", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserService{createErr: errors.New("database down")}, `{"nickname":"Alice","username":"alice","password":"secret"}`)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
		}
		if envelope.Success || envelope.Code != response.CodeInternalError || envelope.Message != "internal server error" {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

func TestUserControllerList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	createdAt := int64(1780048800000)
	listResponse := response.NewPaginatedData([]dto.UserResponse{{UserID: controllerTestUserID, Nickname: "Alice", Username: "alice", Status: domain.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}, response.NewPagination(1, 20, 128))

	t.Run("default pagination", func(t *testing.T) {
		service := &stubUserService{listResponse: response.NewPaginatedData([]dto.UserResponse{}, response.NewPagination(1, 10, 0))}

		status, envelope := executeList(t, service, "/api/v1/users")

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotList.Page != 1 || service.gotList.PageSize != 10 || service.gotList.Offset != 0 || service.gotList.Limit != 10 {
			t.Fatalf("gotList = %#v", service.gotList)
		}
		assertPaginatedEnvelope(t, envelope, 1, 10, 0, 0, 0)
	})

	t.Run("explicit query", func(t *testing.T) {
		service := &stubUserService{listResponse: listResponse}

		status, envelope := executeList(t, service, "/api/v1/users?page=2&page_size=20&nickname=Ali&username=alice&status=100")

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotList.Page != 2 || service.gotList.PageSize != 20 || service.gotList.Offset != 20 || service.gotList.Limit != 20 || service.gotList.Nickname != "Ali" || service.gotList.Username != "alice" {
			t.Fatalf("gotList = %#v", service.gotList)
		}
		if service.gotList.Status == nil || *service.gotList.Status != domain.UserStatusNormal {
			t.Fatalf("status = %#v", service.gotList.Status)
		}
		assertPaginatedEnvelope(t, envelope, 1, 20, 128, 7, 1)
	})

	t.Run("invalid status", func(t *testing.T) {
		status, envelope := executeList(t, &stubUserService{}, "/api/v1/users?status=999")
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != response.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "status", "用户状态", "enum", "用户状态取值不合法，允许值为：100、200、300")
	})
}

type stubUserService struct {
	response       *dto.UserResponse
	err            error
	gotID          uuid.UUID
	createResponse *dto.UserResponse
	createErr      error
	gotCreate      dto.CreateUserRequest
	listResponse   response.PaginatedData[dto.UserResponse]
	listErr        error
	gotList        dto.ListUsersRequest
}

func (s *stubUserService) CreateUser(_ context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	s.gotCreate = req
	if s.createErr != nil {
		return nil, response.FromError(s.createErr)
	}
	return s.createResponse, nil
}

func (s *stubUserService) GetUserByID(_ context.Context, userID uuid.UUID) (*dto.UserResponse, error) {
	s.gotID = userID
	if s.err != nil {
		return nil, response.FromError(s.err)
	}
	return s.response, nil
}

func (s *stubUserService) ListUsers(_ context.Context, req dto.ListUsersRequest) (response.PaginatedData[dto.UserResponse], error) {
	s.gotList = req
	if s.listErr != nil {
		return response.PaginatedData[dto.UserResponse]{}, response.FromError(s.listErr)
	}
	return s.listResponse, nil
}

func executeCreate(t *testing.T, service *stubUserService, body string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	ctl := NewUserController(service, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = int64(len(body))
	ctx.Request = request

	ctl.Create(ctx)

	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}

func executeGetByID(t *testing.T, service *stubUserService, id string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	ctl := NewUserController(service, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+id, nil)
	ctx.Params = gin.Params{{Key: "user_id", Value: id}}

	ctl.GetByID(ctx)

	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}

func executeList(t *testing.T, service *stubUserService, path string) (int, response.Envelope) {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	ctl := NewUserController(service, validator)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)

	ctl.List(ctx)

	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}

func assertPaginatedEnvelope(t *testing.T, envelope response.Envelope, page, pageSize, total, totalPages, itemCount int) {
	t.Helper()
	if !envelope.Success || envelope.Code != response.CodeOK || envelope.Message != response.MessageOK {
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
		if !ok || item["user_id"] != controllerTestUserID || item["nickname"] != "Alice" || item["username"] != "alice" || item["status"] != float64(domain.UserStatusNormal) {
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
	if pagination["page"] != float64(page) || pagination["page_size"] != float64(pageSize) || pagination["total"] != float64(total) || pagination["total_pages"] != float64(totalPages) {
		t.Fatalf("pagination = %#v", pagination)
	}
}

func assertInvalidUserID(t *testing.T, status int, envelope response.Envelope, message string) {
	t.Helper()
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if envelope.Success || envelope.Code != response.CodeValidationFailed || envelope.Message != message || envelope.Message == "invalid user id" {
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
