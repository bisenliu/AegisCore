package validation

import (
	"encoding"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"reflect"
	"testing"
	"time"

	contracterrors "github.com/aegiscore/platform/contract/errors"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func TestValidateStructAndFieldNames(t *testing.T) {
	validator := newTestValidator(t)

	type request struct {
		Name     string `json:"name" validate:"required" label:"姓名"`
		Age      int    `form:"age" validate:"gt=0"`
		TraceID  string `header:"X-Trace-ID" validate:"required"`
		URIID    int64  `uri:"id" validate:"gt=0"`
		QueryID  int64  `query:"query_id" validate:"gt=0"`
		Priority string `json:"json_priority" form:"form_priority" header:"Header-Priority" uri:"uri_priority" query:"query_priority" validate:"required"`
		Hidden   string `json:"-" validate:"-"`
	}

	require.NoError(t, validator.Validate(&request{Name: "aegis", Age: 1, TraceID: "trace", URIID: 1, QueryID: 1, Priority: "high"}))

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
		"name":          {Field: "name", Label: "姓名", Rule: "required"},
		"age":           {Field: "age", Label: "age", Rule: "gt"},
		"X-Trace-ID":    {Field: "X-Trace-ID", Label: "X-Trace-ID", Rule: "required"},
		"id":            {Field: "id", Label: "id", Rule: "gt"},
		"query_id":      {Field: "query_id", Label: "query_id", Rule: "gt"},
		"json_priority": {Field: "json_priority", Label: "json_priority", Rule: "required"},
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

func TestNewRejectsUnsupportedLocale(t *testing.T) {
	validator, err := New(Options{Locale: "en"})
	require.Nil(t, validator)
	require.ErrorContains(t, err, `unsupported validation locale "en"`)

	validator, err = New(Options{})
	require.NoError(t, err)
	require.NotNil(t, validator)
}

func TestNormalizeAndClassifyErrors(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		Age   *int     `json:"age" label:"年龄"`
		Score *float64 `json:"score"`
		Tags  []string `json:"tags"`
		Meta  map[string]string
		Flag  bool
		Name  string
	}

	t.Run("nil error", func(t *testing.T) {
		require.NoError(t, validator.NormalizeError(&request{}, nil))
	})

	t.Run("already normalized", func(t *testing.T) {
		original := &Error{Message: "already normalized", Kind: contracterrors.KindValidation}
		err := validator.NormalizeError(&request{}, original)
		require.Same(t, original, err)
	})

	t.Run("json type", func(t *testing.T) {
		err := validator.NormalizeError(&request{}, &json.UnmarshalTypeError{Field: "age", Type: reflect.TypeOf(int(0))})
		var validationErr *Error
		require.ErrorAs(t, err, &validationErr)
		require.Equal(t, "年龄字段类型不正确，应为整数类型", validationErr.Message)
		require.Equal(t, contracterrors.KindBadRequest, validationErr.Kind)
		require.Equal(t, contracterrors.ReasonRequestBindingFailed, validationErr.Reason)
		require.Equal(t, contracterrors.CodeBadRequest, validationErr.Code)
	})

	t.Run("empty body and unknown error", func(t *testing.T) {
		err := validator.NormalizeError(&request{}, io.EOF)
		var validationErr *Error
		require.ErrorAs(t, err, &validationErr)
		require.Equal(t, ErrEmptyRequestBody, validationErr.Message)
		require.Equal(t, contracterrors.ReasonEmptyRequestBody, validationErr.Reason)

		raw := errors.New("decoder failed")
		require.ErrorIs(t, validator.NormalizeError(&request{}, raw), raw)
	})

	t.Run("classify validation and plain errors", func(t *testing.T) {
		err := validator.Validate(&struct {
			Name string `json:"name" validate:"required" label:"姓名"`
		}{})
		failure := ClassifyError(err)
		require.True(t, failure.IsValidation)
		require.Equal(t, ErrValidationFailed, failure.Message)
		require.NotEmpty(t, failure.Fields)
		require.Equal(t, contracterrors.KindValidation, failure.Kind)
		require.Equal(t, contracterrors.ReasonValidationFailed, failure.Reason)
		require.Equal(t, contracterrors.CodeValidationFailed, failure.Code)

		plain := ClassifyError(errors.New("plain failure"))
		require.False(t, plain.IsValidation)
		require.Equal(t, "plain failure", plain.Message)
		require.Empty(t, plain.Fields)
		require.Empty(t, plain.Kind)
	})

	t.Run("expected type names", func(t *testing.T) {
		cases := []struct {
			err  error
			want string
		}{
			{&json.UnmarshalTypeError{Field: "", Type: nil}, "参数字段类型不正确，应为正确类型"},
			{&json.UnmarshalTypeError{Field: "missing", Type: reflect.TypeOf(uint(0))}, "missing字段类型不正确，应为正整数类型"},
			{&json.UnmarshalTypeError{Field: "score", Type: reflect.TypeOf(float64(0))}, "score字段类型不正确，应为浮点数类型"},
			{&json.UnmarshalTypeError{Field: "Flag", Type: reflect.TypeOf(true)}, "Flag字段类型不正确，应为布尔类型"},
			{&json.UnmarshalTypeError{Field: "Name", Type: reflect.TypeOf("")}, "Name字段类型不正确，应为字符串类型"},
			{&json.UnmarshalTypeError{Field: "tags", Type: reflect.TypeOf([]string{})}, "tags字段类型不正确，应为数组类型"},
			{&json.UnmarshalTypeError{Field: "Meta", Type: reflect.TypeOf(map[string]string{})}, "Meta字段类型不正确，应为映射类型"},
			{&json.UnmarshalTypeError{Field: "age", Type: reflect.PointerTo(reflect.TypeOf(time.Time{}))}, "年龄字段类型不正确，应为time.Time类型"},
		}
		for _, tt := range cases {
			err := validator.NormalizeError(&request{}, tt.err)
			require.EqualError(t, err, tt.want)
		}
	})

	t.Run("default app error metadata", func(t *testing.T) {
		withFields := &Error{Message: "field failed", Fields: []FieldError{{Field: "name"}}}
		require.ErrorIs(t, withFields, &contracterrors.Error{Kind: contracterrors.KindValidation, Reason: contracterrors.ReasonValidationFailed, Code: contracterrors.CodeValidationFailed})

		withoutFields := &Error{Message: "body failed"}
		require.ErrorIs(t, withoutFields, &contracterrors.Error{Kind: contracterrors.KindBadRequest, Reason: contracterrors.ReasonBadRequest, Code: contracterrors.CodeBadRequest})

		internal := &Error{Message: "internal", Kind: contracterrors.KindInternal}
		require.ErrorIs(t, internal, &contracterrors.Error{Kind: contracterrors.KindInternal, Reason: contracterrors.ReasonInternalError, Code: contracterrors.CodeInternalError})
		require.Empty(t, (*Error)(nil).Error())
		require.NoError(t, (*Error)(nil).Unwrap())
	})
}

func TestBindValues(t *testing.T) {
	type nested struct {
		City string `query:"city"`
	}
	type ActiveFilter struct {
		Status *testStatus `query:"status" label:"状态"`
		Min    *int        `query:"min"`
		Deeper *struct {
			Code string `query:"code"`
		}
	}
	type UnusedFilter struct {
		Unused string `query:"unused"`
	}
	type request struct {
		Name     string        `form:"name" label:"姓名"`
		Enabled  bool          `query:"enabled"`
		Count    int           `form:"count,omitempty"`
		Total    uint          `query:"total"`
		Ratio    float64       `form:"ratio"`
		Wait     time.Duration `query:"wait"`
		Tags     []string      `form:"tag"`
		Scores   []int         `query:"score"`
		Skipped  string        `form:"-"`
		Hidden   string        `json:"hidden"`
		private  string        `form:"private"`
		Optional *string       `query:"optional"`
		Blank    *string       `query:"blank"`
		nested
		*ActiveFilter
		*UnusedFilter
		Custom customText     `form:"custom"`
		When   customTextTime `query:"when"`
	}

	values := url.Values{
		"name":     {"aegis"},
		"enabled":  {"true"},
		"count":    {"7"},
		"total":    {"9"},
		"ratio":    {"1.5"},
		"wait":     {"5s"},
		"tag":      {"one", "two"},
		"score":    {"1", "2"},
		"optional": {"set"},
		"blank":    {""},
		"city":     {"shanghai"},
		"status":   {"active"},
		"min":      {"3"},
		"code":     {"CN"},
		"custom":   {"ready"},
		"when":     {"2026-11-23T00:00:00Z"},
		"private":  {"secret"},
		"hidden":   {"ignored"},
		"Skipped":  {"ignored"},
	}
	var got request
	require.NoError(t, BindValues(&got, values, TagForm, TagQuery))
	require.Equal(t, "aegis", got.Name)
	require.True(t, got.Enabled)
	require.Equal(t, 7, got.Count)
	require.Equal(t, uint(9), got.Total)
	require.Equal(t, 1.5, got.Ratio)
	require.Equal(t, 5*time.Second, got.Wait)
	require.Equal(t, []string{"one", "two"}, got.Tags)
	require.Equal(t, []int{1, 2}, got.Scores)
	require.NotNil(t, got.Optional)
	require.Equal(t, "set", *got.Optional)
	require.Nil(t, got.Blank)
	require.Equal(t, "shanghai", got.City)
	require.NotNil(t, got.ActiveFilter)
	require.Equal(t, testStatus("active"), *got.Status)
	require.NotNil(t, got.Min)
	require.Equal(t, 3, *got.Min)
	if got.Deeper != nil {
		require.Equal(t, "CN", got.Deeper.Code)
	}
	require.Nil(t, got.UnusedFilter)
	require.Equal(t, customText("ready"), got.Custom)
	require.True(t, got.When.Equal(time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)))
	require.Empty(t, got.private)
	require.Empty(t, got.Hidden)
	require.Empty(t, got.Skipped)

	t.Run("target and scalar errors", func(t *testing.T) {
		require.ErrorContains(t, BindValues(request{}, values), "non-nil pointer")
		var text string
		require.ErrorContains(t, BindValues(&text, values), "must point to a struct")

		var bad request
		err := BindValues(&bad, url.Values{"count": {"bad"}}, TagForm, TagQuery)
		var bindErr *bindFieldError
		require.ErrorAs(t, err, &bindErr)
		require.Equal(t, "count字段类型不正确，应为整数类型", validatorNormalize(t, &bad, err))
		require.Error(t, BindValues(&bad, url.Values{"enabled": {"maybe"}}, TagQuery))
		require.Error(t, BindValues(&bad, url.Values{"total": {"-1"}}, TagQuery))
		require.Error(t, BindValues(&bad, url.Values{"ratio": {"bad"}}, TagForm))
		require.Error(t, BindValues(&bad, url.Values{"wait": {"later"}}, TagQuery))
		require.Equal(t, "score字段类型不正确，应为数组类型", validatorNormalize(t, &bad, BindValues(&bad, url.Values{"score": {"1", "bad"}}, TagQuery)))
		require.Error(t, BindValues(&bad, url.Values{"custom": {"bad"}}, TagForm))
		require.Error(t, BindValues(&bad, url.Values{"when": {"not-a-time"}}, TagQuery))
	})

	t.Run("unsupported field type", func(t *testing.T) {
		var target struct {
			Meta map[string]string `form:"meta"`
		}
		err := BindValues(&target, url.Values{"meta": {"value"}}, TagForm)
		var bindErr *bindFieldError
		require.ErrorAs(t, err, &bindErr)
		require.ErrorContains(t, bindErr.Unwrap(), "unsupported field type")
		require.NotEmpty(t, bindErr.Error())
	})
}

func TestEnumPointerAndTranslationBoundaries(t *testing.T) {
	validator := newTestValidator(t)

	t.Run("direct helpers reject invalid values", func(t *testing.T) {
		require.False(t, validateEnum(fakeFieldLevel{value: reflect.Value{}}))
		var status *testStatus
		require.False(t, validateEnum(fakeFieldLevel{value: reflect.ValueOf(status)}))
		require.Nil(t, enumAllowedValues(nil))
		require.Nil(t, enumAllowedValues(status))
		statusValue := testStatus("active")
		require.Equal(t, []string{"active", "inactive"}, enumAllowedValues(&statusValue))

		msg, err := newTestValidator(t).trans.T("missing-enum-rule", "状态")
		require.Error(t, err)
		require.Empty(t, msg)
	})

	t.Run("pointer value and nil allowed values", func(t *testing.T) {
		status := testStatus("active")
		require.NoError(t, validator.Validate(&pointerEnumRequest{Status: &status}))

		invalid := testStatus("disabled")
		err := validator.Validate(&pointerEnumRequest{Status: &invalid})
		var validationErr *Error
		require.ErrorAs(t, err, &validationErr)
		require.Contains(t, validationErr.Fields[0].Message, "允许值为：active、inactive")

		nilStatus := pointerEnumWithoutValuesRequest{}
		assertNoPanic(t, func() {
			require.Error(t, validator.Validate(&nilStatus))
		})
	})

	t.Run("nested and slice field paths", func(t *testing.T) {
		type child struct {
			Name string `json:"name" validate:"required" label:"姓名"`
		}
		type parent struct {
			Child  child    `json:"child"`
			Items  []child  `json:"items" validate:"dive"`
			Hidden string   `json:"-" validate:"required"`
			Raw    struct{} `validate:"required"`
		}
		err := validator.Validate(&parent{Items: []child{{}}})
		var validationErr *Error
		require.ErrorAs(t, err, &validationErr)
		fields := fieldDetails(validationErr.Fields)
		require.Equal(t, "姓名", fields["child.name"].Label)
		require.Equal(t, "姓名", fields["items.name"].Label)
		require.NotEmpty(t, fields["Hidden"].Message)
	})

	t.Run("display name fallbacks", func(t *testing.T) {
		type child struct {
			Nickname string `json:"nickname" label:"昵称"`
		}
		type parent struct {
			Child *child `json:"child"`
			Note  string `json:"note,omitempty"`
		}
		require.Equal(t, "参数", displayName(nil, ""))
		require.Equal(t, "missing.path", displayName("not-struct", "missing.path"))
		require.Equal(t, "昵称", displayName(&parent{}, "child.nickname"))
		require.Equal(t, "note", displayName(parent{}, "note.extra"))
		require.Equal(t, "missing", displayName(&parent{}, "missing"))
		require.Equal(t, "parent", displayNameFromType(reflect.TypeOf(parent{}), nil))
	})
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

type pointerEnumWithoutValuesRequest struct {
	Status *testStatusWithoutValues `validate:"enum"`
}

type customText string

func (c *customText) UnmarshalText(text []byte) error {
	if string(text) == "bad" {
		return errors.New("custom text rejected")
	}
	*c = customText(text)
	return nil
}

var _ encoding.TextUnmarshaler = (*customText)(nil)

type customTextTime time.Time

func (c *customTextTime) UnmarshalText(text []byte) error {
	parsed, err := time.Parse(time.RFC3339, string(text))
	if err != nil {
		return err
	}
	*c = customTextTime(parsed)
	return nil
}

func (c customTextTime) Equal(other time.Time) bool {
	return time.Time(c).Equal(other)
}

type misconfiguredEnumRequest struct {
	Status string `validate:"enum"`
}

func validatorNormalize(t *testing.T, dst any, err error) string {
	t.Helper()
	normalized := newTestValidator(t).NormalizeError(dst, err)
	require.Error(t, normalized)
	return normalized.Error()
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

type fakeFieldLevel struct {
	validator.FieldLevel
	value reflect.Value
}

func (f fakeFieldLevel) Field() reflect.Value {
	return f.value
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	require.NotPanics(t, fn)
}
