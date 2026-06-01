package validation

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
)

func TestValidateStructAndFieldNames(t *testing.T) {
	validator := newTestValidator(t)

	type request struct {
		Name    string `json:"name" validate:"required" label:"姓名"`
		Age     int    `form:"age" validate:"gt=0"`
		URIID   int64  `uri:"id" validate:"gt=0"`
		QueryID int64  `query:"query_id" validate:"gt=0"`
		Hidden  string `json:"-" validate:"-"`
	}

	if err := validator.Validate(&request{Name: "aegis", Age: 1, URIID: 1, QueryID: 1}); err != nil {
		t.Fatalf("Validate valid request: %v", err)
	}

	err := validator.Validate(&request{})
	var validationErr *Error
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate error = %T, want *Error", err)
	}
	if validationErr.Message != ErrValidationFailed {
		t.Fatalf("Message = %q, want %q", validationErr.Message, ErrValidationFailed)
	}
	if validationErr.Code != response.CodeValidationFailed {
		t.Fatalf("Code = %d, want %d", validationErr.Code, response.CodeValidationFailed)
	}
	fields := fieldDetails(validationErr.Fields)
	checks := map[string]FieldError{
		"name":     {Field: "name", Label: "姓名", Rule: "required"},
		"age":      {Field: "age", Label: "age", Rule: "gt"},
		"id":       {Field: "id", Label: "id", Rule: "gt"},
		"query_id": {Field: "query_id", Label: "query_id", Rule: "gt"},
	}
	for field, want := range checks {
		got := fields[field]
		if got.Message == "" {
			t.Fatalf("missing field error for %q in %#v", field, validationErr.Fields)
		}
		if got.Label != want.Label || got.Rule != want.Rule {
			t.Fatalf("field error for %q = %#v, want label %q rule %q", field, got, want.Label, want.Rule)
		}
	}
	if fields["Hidden"].Message != "" || fields["-"].Message != "" {
		t.Fatalf("hidden field leaked in %#v", validationErr.Fields)
	}
}

func TestValidateEmailFieldDetails(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		Email string `json:"email" validate:"required,email" label:"邮箱"`
	}

	err := validator.Validate(&request{Email: "bad"})
	var validationErr *Error
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate error = %T, want *Error", err)
	}
	if len(validationErr.Fields) != 1 {
		t.Fatalf("Fields = %#v, want one field", validationErr.Fields)
	}
	field := validationErr.Fields[0]
	if field.Field != "email" || field.Label != "邮箱" || field.Rule != "email" || field.Message != "邮箱格式不正确" {
		t.Fatalf("field = %#v", field)
	}
}

func TestJSONBinder(t *testing.T) {
	validator := newTestValidator(t)

	type request struct {
		ID int64 `json:"id" validate:"required,gt=0" label:"用户ID"`
	}

	t.Run("valid body", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123}`)
		var req request
		if err := validator.Bind(ctx, &req, JSONBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.ID != 123 {
			t.Fatalf("ID = %d, want 123", req.ID)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		ctx := newJSONContext("")
		var req request
		err := validator.Bind(ctx, &req, JSONBinder)
		var validationErr *Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Bind error = %T, want *Error", err)
		}
		if validationErr.Message != ErrEmptyRequestBody {
			t.Fatalf("Message = %q, want %q", validationErr.Message, ErrEmptyRequestBody)
		}
		if validationErr.Code != response.CodeBadRequest {
			t.Fatalf("Code = %d, want %d", validationErr.Code, response.CodeBadRequest)
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		ctx := newJSONContext(`{"id":"bad"}`)
		var req request
		err := validator.Bind(ctx, &req, JSONBinder)
		var validationErr *Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Bind error = %T, want *Error", err)
		}
		if got, want := validationErr.Message, "用户ID字段类型不正确，应为整数类型"; got != want {
			t.Fatalf("Message = %q, want %q", got, want)
		}
		if validationErr.Code != response.CodeBadRequest {
			t.Fatalf("Code = %d, want %d", validationErr.Code, response.CodeBadRequest)
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
		if err := validator.Bind(ctx, &req, URIBinder); err != nil {
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
		err := validator.Bind(ctx, &req, URIBinder)
		var validationErr *Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Bind error = %T, want *Error", err)
		}
		if got, want := validationErr.Message, "用户ID字段类型不正确，应为整数类型"; got != want {
			t.Fatalf("Message = %q, want %q", got, want)
		}
		if validationErr.Code != response.CodeBadRequest {
			t.Fatalf("Code = %d, want %d", validationErr.Code, response.CodeBadRequest)
		}
	})

	t.Run("query", func(t *testing.T) {
		type request struct {
			Page int `query:"page" validate:"required,gt=0"`
		}
		ctx := newRequestContext(http.MethodGet, "/users?page=2", "")
		var req request
		if err := validator.Bind(ctx, &req, QueryBinder); err != nil {
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
		if err := validator.Bind(ctx, &req, FormBinder); err != nil {
			t.Fatalf("Bind: %v", err)
		}
		if req.Name != "aegis" {
			t.Fatalf("Name = %q, want aegis", req.Name)
		}
	})
}

func TestExtensionHooks(t *testing.T) {
	validator := newTestValidator(t)

	t.Run("defaults before validation", func(t *testing.T) {
		req := &defaultableRequest{}
		if err := validator.Validate(req); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if req.Limit != 20 {
			t.Fatalf("Limit = %d, want 20", req.Limit)
		}
	})

	t.Run("custom validation", func(t *testing.T) {
		req := &customRequest{Name: "bad"}
		err := validator.Validate(req)
		if err == nil || err.Error() != "name is not allowed" {
			t.Fatalf("Validate error = %v, want custom error", err)
		}
	})
}

func TestEnumValidation(t *testing.T) {
	validator := newTestValidator(t)

	t.Run("valid", func(t *testing.T) {
		req := enumRequest{Status: testStatus("active")}
		if err := validator.Validate(&req); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		req := enumRequest{Status: testStatus("disabled")}
		if err := validator.Validate(&req); err == nil {
			t.Fatal("Validate error = nil, want enum error")
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		req := pointerEnumRequest{}
		assertNoPanic(t, func() {
			if err := validator.Validate(&req); err == nil {
				t.Fatal("Validate error = nil, want enum error")
			}
		})
	})

	t.Run("misconfigured", func(t *testing.T) {
		req := misconfiguredEnumRequest{Status: "active"}
		assertNoPanic(t, func() {
			if err := validator.Validate(&req); err == nil {
				t.Fatal("Validate error = nil, want enum error")
			}
		})
	})
}

func TestBindOrAbort(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		Name string `json:"name" validate:"required"`
	}
	ctx, recorder := newJSONContextWithRecorder(`{}`)
	var req request
	if validator.BindOrAbort(ctx, &req, JSONBinder) {
		t.Fatal("BindOrAbort = true, want false")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var envelope response.Envelope
	if err := jsonUnmarshal(recorder.Body.String(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != response.CodeValidationFailed || envelope.Message != ErrValidationFailed {
		t.Fatalf("envelope = %#v", envelope)
	}
	if len(envelope.Errors.([]any)) != 1 {
		t.Fatalf("errors = %#v, want one error", envelope.Errors)
	}
}

func TestBindOrAbortTypeMismatchUsesBadRequest(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		ID int64 `uri:"id" label:"用户ID"`
	}
	ctx, recorder := newRequestContextWithRecorder(http.MethodGet, "/users/bad", "")
	ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
	var req request
	if validator.BindOrAbort(ctx, &req, URIBinder) {
		t.Fatal("BindOrAbort = true, want false")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var envelope response.Envelope
	if err := jsonUnmarshal(recorder.Body.String(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Success || envelope.Code != response.CodeBadRequest || envelope.Message != "用户ID字段类型不正确，应为整数类型" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if envelope.Errors != nil {
		t.Fatalf("errors = %#v, want nil", envelope.Errors)
	}
}

type defaultableRequest struct {
	Limit int `validate:"required,gt=0"`
}

func (r *defaultableRequest) SetDefaults() {
	if r.Limit == 0 {
		r.Limit = 20
	}
}

type customRequest struct {
	Name string `validate:"required"`
}

func (r *customRequest) Validate() error {
	if r.Name == "bad" {
		return errors.New("name is not allowed")
	}
	return nil
}

type testStatus string

func (s testStatus) IsValid() bool {
	return s == "active"
}

type enumRequest struct {
	Status testStatus `validate:"enum"`
}

type pointerEnumRequest struct {
	Status *testStatus `validate:"enum"`
}

type misconfiguredEnumRequest struct {
	Status string `validate:"enum"`
}

func newTestValidator(t *testing.T) *Validator {
	t.Helper()
	validator, err := NewDefault()
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

func fieldDetails(fields []FieldError) map[string]FieldError {
	messages := make(map[string]FieldError, len(fields))
	for _, field := range fields {
		messages[field.Field] = field
	}
	return messages
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	fn()
}

func jsonUnmarshal(raw string, dst any) error {
	return json.NewDecoder(strings.NewReader(raw)).Decode(dst)
}
