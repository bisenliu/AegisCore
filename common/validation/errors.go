package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/aegiscore/common/response"
	"github.com/go-playground/validator/v10"
)

func (v *Validator) normalizeError(dst any, err error) error {
	if err == nil {
		return nil
	}
	var validationErr *Error
	if errors.As(err, &validationErr) {
		return validationErr
	}
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		fields := make([]FieldError, 0, len(validationErrors))
		for _, fieldErr := range validationErrors {
			field, label := validationFieldNames(dst, fieldErr)
			fields = append(fields, FieldError{Field: field, Label: label, Rule: fieldErr.Tag(), Message: fieldErr.Translate(v.trans)})
		}
		return &Error{Message: ErrValidationFailed, Fields: fields, Code: response.CodeValidationFailed}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := displayName(dst, typeErr.Field)
		return &Error{Message: fmt.Sprintf("%s字段类型不正确，应为%s类型", field, expectedType(typeErr.Type)), Code: response.CodeBadRequest}
	}
	var bindErr *bindFieldError
	if errors.As(err, &bindErr) {
		return &Error{Message: fmt.Sprintf("%s字段类型不正确，应为%s类型", bindErr.field, expectedType(bindErr.typ)), Code: response.CodeBadRequest}
	}
	if errors.Is(err, io.EOF) {
		return &Error{Message: ErrEmptyRequestBody, Code: response.CodeBadRequest}
	}
	return err
}

func publicMessage(err error) string {
	var validationErr *Error
	if errors.As(err, &validationErr) && validationErr.Message != "" {
		return validationErr.Message
	}
	return err.Error()
}

func validationFailure(err error) bool {
	var validationErr *Error
	return errors.As(err, &validationErr) && validationErr.Code == response.CodeValidationFailed
}

func validationDetails(err error) []FieldError {
	var validationErr *Error
	if errors.As(err, &validationErr) {
		return validationErr.Fields
	}
	return nil
}

func expectedType(t reflect.Type) string {
	if t == nil {
		return "正确"
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "整数"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "正整数"
	case reflect.Float32, reflect.Float64:
		return "浮点数"
	case reflect.Bool:
		return "布尔"
	case reflect.String:
		return "字符串"
	case reflect.Array, reflect.Slice:
		return "数组"
	case reflect.Map:
		return "映射"
	default:
		return t.String()
	}
}
