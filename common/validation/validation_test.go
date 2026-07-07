package validation

import (
	"errors"
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"

	"github.com/stretchr/testify/require"
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

	require.NoError(t, validator.Validate(&request{Name: "aegis", Age: 1, URIID: 1, QueryID: 1}))

	err := validator.Validate(&request{})
	var validationErr *Error
	require.ErrorAs(t, err, &validationErr)
	require.Equal(t, ErrValidationFailed, validationErr.Message)
	require.Equal(t, contracterrors.KindValidation, validationErr.Kind)
	require.Equal(t, contracterrors.ReasonValidationFailed, validationErr.Reason)
	require.Equal(t, contracterrors.CodeValidationFailed, validationErr.Code)
	require.ErrorIs(t, validationErr, &contracterrors.Error{Kind: contracterrors.KindValidation})
	fields := fieldDetails(validationErr.Fields)
	checks := map[string]FieldError{
		"name":     {Field: "name", Label: "姓名", Rule: "required"},
		"age":      {Field: "age", Label: "age", Rule: "gt"},
		"id":       {Field: "id", Label: "id", Rule: "gt"},
		"query_id": {Field: "query_id", Label: "query_id", Rule: "gt"},
	}
	for field, want := range checks {
		got := fields[field]
		require.NotEmpty(t, got.Message, "missing field error for %q", field)
		require.Equal(t, want.Label, got.Label)
		require.Equal(t, want.Rule, got.Rule)
	}
	require.Empty(t, fields["Hidden"].Message)
	require.Empty(t, fields["-"].Message)
}

func TestValidateEmailFieldDetails(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		Email string `json:"email" validate:"required,email" label:"邮箱"`
	}

	err := validator.Validate(&request{Email: "bad"})
	var validationErr *Error
	require.ErrorAs(t, err, &validationErr)
	require.Len(t, validationErr.Fields, 1)
	field := validationErr.Fields[0]
	require.Equal(t, "email", field.Field)
	require.Equal(t, "邮箱", field.Label)
	require.Equal(t, "email", field.Rule)
	require.Equal(t, "邮箱必须是一个有效的邮箱", field.Message)
}

func TestExtensionHooks(t *testing.T) {
	validator := newTestValidator(t)

	t.Run("defaults before validation", func(t *testing.T) {
		req := &defaultableRequest{}
		require.NoError(t, validator.Validate(req))
		require.Equal(t, 20, req.Limit)
	})

	t.Run("custom validation", func(t *testing.T) {
		req := &customRequest{Name: "bad"}
		err := validator.Validate(req)
		require.EqualError(t, err, "name is not allowed")
	})
}

func TestEnumValidation(t *testing.T) {
	validator := newTestValidator(t)

	t.Run("valid", func(t *testing.T) {
		req := enumRequest{Status: testStatus("active")}
		require.NoError(t, validator.Validate(&req))
	})

	t.Run("invalid", func(t *testing.T) {
		req := enumRequest{Status: testStatus("disabled")}
		err := validator.Validate(&req)
		var validationErr *Error
		require.ErrorAs(t, err, &validationErr)
		fields := fieldDetails(validationErr.Fields)
		require.Equal(t, "用户状态取值不合法，允许值为：active、inactive", fields["status"].Message)
	})

	t.Run("invalid without allowed values", func(t *testing.T) {
		req := enumWithoutValuesRequest{Status: testStatusWithoutValues("disabled")}
		err := validator.Validate(&req)
		var validationErr *Error
		require.ErrorAs(t, err, &validationErr)
		fields := fieldDetails(validationErr.Fields)
		require.Equal(t, "用户状态取值不合法", fields["status"].Message)
	})

	t.Run("nil pointer", func(t *testing.T) {
		req := pointerEnumRequest{}
		assertNoPanic(t, func() {
			require.Error(t, validator.Validate(&req))
		})
	})

	t.Run("misconfigured", func(t *testing.T) {
		req := misconfiguredEnumRequest{Status: "active"}
		assertNoPanic(t, func() {
			require.Error(t, validator.Validate(&req))
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
	require.NoError(t, err)
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
	require.NotPanics(t, fn)
}
