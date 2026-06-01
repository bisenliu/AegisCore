package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/apperror"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/gin-gonic/gin"
)

func TestUserControllerGetByID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid ID", func(t *testing.T) {
		createdAt := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(time.Hour)
		service := &stubUserService{response: &dto.UserResponse{
			ID:        123,
			Name:      "Aegis",
			Email:     "aegis@example.com",
			Active:    true,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}}

		status, envelope := executeGetByID(t, service, "123")

		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if service.gotID != 123 {
			t.Fatalf("gotID = %d, want 123", service.gotID)
		}
		if !envelope.Success || envelope.Code != response.CodeOK || envelope.Message != "ok" {
			t.Fatalf("envelope = %#v", envelope)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok {
			t.Fatalf("data = %T, want map", envelope.Data)
		}
		if data["id"] != float64(123) || data["name"] != "Aegis" {
			t.Fatalf("data = %#v", data)
		}
	})

	t.Run("non numeric ID", func(t *testing.T) {
		status, envelope := executeGetByID(t, &stubUserService{}, "abc")
		assertInvalidUserID(t, status, envelope, "用户ID字段类型不正确，应为整数类型")
	})

	t.Run("non positive ID", func(t *testing.T) {
		status, envelope := executeGetByID(t, &stubUserService{}, "0")
		assertInvalidUserID(t, status, envelope, validation.ErrValidationFailed)
	})

	t.Run("not found", func(t *testing.T) {
		status, envelope := executeGetByID(t, &stubUserService{err: response.NotFoundError(apperror.MsgUserNotFound)}, "999")
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", status, http.StatusNotFound)
		}
		if envelope.Success || envelope.Code != response.CodeNotFound || envelope.Message != apperror.MsgUserNotFound {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("service error", func(t *testing.T) {
		status, envelope := executeGetByID(t, &stubUserService{err: errors.New("database down")}, "123")
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

	createdAt := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	createdUser := &dto.UserResponse{
		ID:        123,
		Name:      "Alice",
		Email:     "alice@example.com",
		Active:    true,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	t.Run("valid body", func(t *testing.T) {
		service := &stubUserService{createResponse: createdUser}

		status, envelope := executeCreate(t, service, `{"name":"Alice","email":"alice@example.com"}`)

		if status != http.StatusCreated {
			t.Fatalf("status = %d, want %d", status, http.StatusCreated)
		}
		if service.gotCreate.Email != "alice@example.com" {
			t.Fatalf("gotCreate = %#v", service.gotCreate)
		}
		if !envelope.Success || envelope.Code != response.CodeOK || envelope.Message != "created" {
			t.Fatalf("envelope = %#v", envelope)
		}
		data, ok := envelope.Data.(map[string]any)
		if !ok || data["id"] != float64(123) || data["email"] != "alice@example.com" {
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
		status, envelope := executeCreate(t, &stubUserService{}, `{"name":"Alice","email":"bad"}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
		}
		if envelope.Success || envelope.Code != response.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
			t.Fatalf("envelope = %#v", envelope)
		}
		assertFieldError(t, envelope, "email", "邮箱", "email", "邮箱必须是一个有效的邮箱")
	})

	t.Run("user already exists", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserService{createErr: response.ConflictError(apperror.MsgUserAlreadyExists)}, `{"name":"Alice","email":"alice@example.com"}`)
		if status != http.StatusConflict {
			t.Fatalf("status = %d, want %d", status, http.StatusConflict)
		}
		if envelope.Success || envelope.Code != response.CodeConflict || envelope.Message != apperror.MsgUserAlreadyExists {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("service error", func(t *testing.T) {
		status, envelope := executeCreate(t, &stubUserService{createErr: errors.New("database down")}, `{"name":"Alice","email":"alice@example.com"}`)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
		}
		if envelope.Success || envelope.Code != response.CodeInternalError || envelope.Message != "internal server error" {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

type stubUserService struct {
	response       *dto.UserResponse
	err            error
	gotID          int64
	createResponse *dto.UserResponse
	createErr      error
	gotCreate      dto.CreateUserRequest
}

func (s *stubUserService) CreateUser(_ context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	s.gotCreate = req
	if s.createErr != nil {
		return nil, response.FromError(s.createErr)
	}
	return s.createResponse, nil
}

func (s *stubUserService) GetUserByID(_ context.Context, id int64) (*dto.UserResponse, error) {
	s.gotID = id
	if s.err != nil {
		return nil, response.FromError(s.err)
	}
	return s.response, nil
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
	ctx.Params = gin.Params{{Key: "id", Value: id}}

	ctl.GetByID(ctx)

	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return recorder.Code, envelope
}

func assertInvalidUserID(t *testing.T, status int, envelope response.Envelope, message string) {
	t.Helper()
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	wantCode := response.CodeBadRequest
	if message == validation.ErrValidationFailed {
		wantCode = response.CodeValidationFailed
	}
	if envelope.Success || envelope.Code != wantCode || envelope.Message != message || envelope.Message == "invalid user id" {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func assertFieldError(t *testing.T, envelope response.Envelope, field, label, rule, message string) {
	t.Helper()
	errors, ok := envelope.Errors.([]any)
	if !ok || len(errors) != 1 {
		t.Fatalf("errors = %#v, want one error", envelope.Errors)
	}
	got, ok := errors[0].(map[string]any)
	if !ok {
		t.Fatalf("field error = %T, want map", errors[0])
	}
	if got["field"] != field || got["label"] != label || got["rule"] != rule || got["message"] != message {
		t.Fatalf("field error = %#v", got)
	}
}
