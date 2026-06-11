package binding

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/validation"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestJSONBinder(t *testing.T) {
	validator := newTestValidator(t)

	type request struct {
		ID int64 `json:"id" validate:"required,gt=0" label:"用户ID"`
	}

	t.Run("valid body", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123}`)
		var req request
		if err := Bind(validator, ctx, &req, JSONBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.ID != 123 {
			t.Fatalf("ID = %d, want 123", req.ID)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		ctx := newJSONContext("")
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var validationErr *validation.Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Bind error = %T, want *Error", err)
		}
		if validationErr.Message != validation.ErrEmptyRequestBody {
			t.Fatalf("Message = %q, want %q", validationErr.Message, validation.ErrEmptyRequestBody)
		}
		if validationErr.Code != contracterrors.CodeBadRequest {
			t.Fatalf("Code = %d, want %d", validationErr.Code, contracterrors.CodeBadRequest)
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		ctx := newJSONContext(`{"id":"bad"}`)
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var validationErr *validation.Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Bind error = %T, want *Error", err)
		}
		if got, want := validationErr.Message, "用户ID字段类型不正确，应为整数类型"; got != want {
			t.Fatalf("Message = %q, want %q", got, want)
		}
		if validationErr.Code != contracterrors.CodeBadRequest {
			t.Fatalf("Code = %d, want %d", validationErr.Code, contracterrors.CodeBadRequest)
		}
	})

	t.Run("trailing body", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123} {"id":456}`)
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var validationErr *validation.Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Bind error = %T, want *Error", err)
		}
		if validationErr.Message != validation.ErrTrailingJSONBody || validationErr.Code != contracterrors.CodeBadRequest {
			t.Fatalf("validation error = %#v", validationErr)
		}
	})

	t.Run("unknown field compatible by default", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123,"extra":true}`)
		var req request
		if err := Bind(validator, ctx, &req, JSONBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
	})

	t.Run("unknown field rejected in strict mode", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123,"extra":true}`)
		var req request
		if err := Bind(validator, ctx, &req, StrictJSONBinder); err == nil {
			t.Fatal("Bind error = nil, want unknown field error")
		}
	})
}

func TestBinders(t *testing.T) {
	validator := newTestValidator(t)

	t.Run("uri", func(t *testing.T) {
		type request struct {
			ID int64 `uri:"id" validate:"required,gt=0"`
		}
		ctx := newRequestContext(http.MethodGet, "/users/123", "")
		ctx.Params = gin.Params{{Key: "id", Value: "123"}}
		var req request
		if err := Bind(validator, ctx, &req, URIBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.ID != 123 {
			t.Fatalf("ID = %d, want 123", req.ID)
		}
	})

	t.Run("uri type mismatch", func(t *testing.T) {
		type request struct {
			ID int64 `uri:"id" validate:"required,gt=0" label:"用户ID"`
		}
		ctx := newRequestContext(http.MethodGet, "/users/bad", "")
		ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
		var req request
		err := Bind(validator, ctx, &req, URIBinder)
		var validationErr *validation.Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Bind error = %T, want *Error", err)
		}
		if got, want := validationErr.Message, "用户ID字段类型不正确，应为整数类型"; got != want {
			t.Fatalf("Message = %q, want %q", got, want)
		}
		if validationErr.Code != contracterrors.CodeBadRequest {
			t.Fatalf("Code = %d, want %d", validationErr.Code, contracterrors.CodeBadRequest)
		}
	})

	t.Run("query", func(t *testing.T) {
		type request struct {
			Page int `query:"page" validate:"required,gt=0"`
		}
		ctx := newRequestContext(http.MethodGet, "/users?page=2", "")
		var req request
		if err := Bind(validator, ctx, &req, QueryBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.Page != 2 {
			t.Fatalf("Page = %d, want 2", req.Page)
		}
	})

	t.Run("form", func(t *testing.T) {
		type request struct {
			Name string `form:"name" validate:"required"`
		}
		ctx := newRequestContext(http.MethodPost, "/users", "name=aegis")
		ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var req request
		if err := Bind(validator, ctx, &req, FormBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.Name != "aegis" {
			t.Fatalf("Name = %q, want aegis", req.Name)
		}
	})

	t.Run("custom text unmarshaler and duration", func(t *testing.T) {
		type request struct {
			Status testTextValue `query:"status"`
			TTL    time.Duration `query:"ttl"`
		}
		ctx := newRequestContext(http.MethodGet, "/users?status=active&ttl=5s", "")
		var req request
		if err := Bind(validator, ctx, &req, QueryBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.Status != testTextValue("parsed:active") || req.TTL != 5*time.Second {
			t.Fatalf("request = %#v", req)
		}
	})

	t.Run("embedded pointer struct", func(t *testing.T) {
		type request struct {
			*TestEmbedded
		}
		ctx := newRequestContext(http.MethodGet, "/users?page=3", "")
		var req request
		if err := Bind(validator, ctx, &req, QueryBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.TestEmbedded == nil || req.Page != 3 {
			t.Fatalf("embedded request = %#v", req)
		}
	})
}

func TestBindOrAbort(t *testing.T) {
	validator := newTestValidator(t)
	logs := captureValidationLogs(t)
	type request struct {
		Name string `json:"name" validate:"required"`
	}
	ctx, recorder := newJSONContextWithRecorder(`{}`)
	var req request
	if BindOrAbort(validator, ctx, &req, JSONBinder) {
		t.Fatal("BindOrAbort = true, want false")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var envelope response.Envelope
	if err := jsonUnmarshal(recorder.Body.String(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeValidationFailed || envelope.Message != validation.ErrValidationFailed {
		t.Fatalf("envelope = %#v", envelope)
	}
	if len(envelope.Errors.([]any)) != 1 {
		t.Fatalf("errors = %#v, want one error", envelope.Errors)
	}
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Level != zapcore.ErrorLevel || entry.Message != "invalid request" {
		t.Fatalf("log entry = level %s message %q, want error invalid request", entry.Level, entry.Message)
	}
	fields := entry.ContextMap()
	if fields["path"] != "/" {
		t.Fatalf("log path = %#v, want /", fields["path"])
	}
	if fields["error"] == nil {
		t.Fatalf("log error field missing: %#v", fields)
	}
	errorsField, ok := fields["errors"].([]validation.FieldError)
	if !ok || len(errorsField) != 1 {
		t.Fatalf("log errors = %#v, want one FieldError", fields["errors"])
	}
	if errorsField[0].Field != "name" || errorsField[0].Rule != "required" {
		t.Fatalf("log field error = %#v", errorsField[0])
	}
}

func TestBindOrAbortTypeMismatchUsesBadRequest(t *testing.T) {
	validator := newTestValidator(t)
	logs := captureValidationLogs(t)
	type request struct {
		ID int64 `uri:"id" label:"用户ID"`
	}
	ctx, recorder := newRequestContextWithRecorder(http.MethodGet, "/users/bad", "")
	ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
	var req request
	if BindOrAbort(validator, ctx, &req, URIBinder) {
		t.Fatal("BindOrAbort = true, want false")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var envelope response.Envelope
	if err := jsonUnmarshal(recorder.Body.String(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != contracterrors.CodeBadRequest || envelope.Message != "用户ID字段类型不正确，应为整数类型" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if envelope.Errors != nil {
		t.Fatalf("errors = %#v, want nil", envelope.Errors)
	}
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Level != zapcore.ErrorLevel || entry.Message != "invalid request" {
		t.Fatalf("log entry = level %s message %q, want error invalid request", entry.Level, entry.Message)
	}
	fields := entry.ContextMap()
	if fields["path"] != "/users/bad" {
		t.Fatalf("log path = %#v, want /users/bad", fields["path"])
	}
	if fields["error"] == nil {
		t.Fatalf("log error field missing: %#v", fields)
	}
	if _, ok := fields["errors"]; ok {
		t.Fatalf("log errors = %#v, want omitted", fields["errors"])
	}
}

type testTextValue string

func (v *testTextValue) UnmarshalText(text []byte) error {
	*v = testTextValue("parsed:" + string(text))
	return nil
}

type TestEmbedded struct {
	Page int `query:"page"`
}

func newTestValidator(t *testing.T) *validation.Validator {
	t.Helper()
	validator, err := validation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault: %v", err)
	}
	return validator
}

func newJSONContext(body string) *gin.Context {
	ctx, _ := newJSONContextWithRecorder(body)
	return ctx
}

func newJSONContextWithRecorder(body string) (*gin.Context, *httptest.ResponseRecorder) {
	ctx, recorder := newRequestContextWithRecorder(http.MethodPost, "/", body)
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func newRequestContext(method, target, body string) *gin.Context {
	ctx, _ := newRequestContextWithRecorder(method, target, body)
	return ctx
}

func newRequestContextWithRecorder(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.ContentLength = int64(len(body))
	ctx.Request = request
	return ctx, recorder
}

func jsonUnmarshal(raw string, dst any) error {
	return json.NewDecoder(strings.NewReader(raw)).Decode(dst)
}

func captureValidationLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	logger.SetDefault(zap.New(core))
	t.Cleanup(func() { logger.SetDefault(nil) })
	return logs
}
