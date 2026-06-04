package validation

import (
	"errors"
	"testing"

	"github.com/aegiscore/common/response"
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
	if field.Field != "email" || field.Label != "邮箱" || field.Rule != "email" || field.Message != "邮箱必须是一个有效的邮箱" {
		t.Fatalf("field = %#v", field)
	}
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
		err := validator.Validate(&req)
		var validationErr *Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Validate error = %T, want *Error", err)
		}
		fields := fieldDetails(validationErr.Fields)
		if got, want := fields["status"].Message, "用户状态取值不合法，允许值为：active、inactive"; got != want {
			t.Fatalf("message = %q, want %q", got, want)
		}
	})

	t.Run("invalid without allowed values", func(t *testing.T) {
		req := enumWithoutValuesRequest{Status: testStatusWithoutValues("disabled")}
		err := validator.Validate(&req)
		var validationErr *Error
		if !errors.As(err, &validationErr) {
			t.Fatalf("Validate error = %T, want *Error", err)
		}
		fields := fieldDetails(validationErr.Fields)
		if got, want := fields["status"].Message, "用户状态取值不合法"; got != want {
			t.Fatalf("message = %q, want %q", got, want)
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

func (s testStatus) AllowedValues() []string {
	return []string{"active", "inactive"}
}

type testStatusWithoutValues string

func (s testStatusWithoutValues) IsValid() bool {
	return s == "active"
}

type enumRequest struct {
	Status testStatus `json:"status" validate:"enum" label:"用户状态"`
}

type enumWithoutValuesRequest struct {
	Status testStatusWithoutValues `json:"status" validate:"enum" label:"用户状态"`
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
