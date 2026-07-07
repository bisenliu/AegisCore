package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/go-playground/validator/v10"

	contracterrors "github.com/aegiscore/common/contract/errors"
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
		return &Error{Message: ErrValidationFailed, Fields: fields, Kind: contracterrors.KindValidation, Reason: contracterrors.ReasonValidationFailed, Code: contracterrors.CodeValidationFailed}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := displayName(dst, typeErr.Field)
		return &Error{Message: fmt.Sprintf("%s字段类型不正确，应为%s类型", field, expectedType(typeErr.Type)), Kind: contracterrors.KindBadRequest, Reason: contracterrors.ReasonRequestBindingFailed, Code: contracterrors.CodeBadRequest}
	}
	var bindErr *bindFieldError
	if errors.As(err, &bindErr) {
		return &Error{Message: fmt.Sprintf("%s字段类型不正确，应为%s类型", bindErr.field, expectedType(bindErr.typ)), Kind: contracterrors.KindBadRequest, Reason: contracterrors.ReasonRequestBindingFailed, Code: contracterrors.CodeBadRequest}
	}
	if errors.Is(err, io.EOF) {
		return &Error{Message: ErrEmptyRequestBody, Kind: contracterrors.KindBadRequest, Reason: contracterrors.ReasonEmptyRequestBody, Code: contracterrors.CodeBadRequest}
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
	var appErr *contracterrors.Error
	return errors.As(err, &appErr) && appErr.Kind == contracterrors.KindValidation
}

func validationDetails(err error) []FieldError {
	var validationErr *Error
	if errors.As(err, &validationErr) {
		return validationErr.Fields
	}
	return nil
}

// ClassifyError 将规范化错误转换为 HTTP handler 使用的响应元数据。
func ClassifyError(err error) Failure {
	failure := Failure{Message: publicMessage(err), Fields: validationDetails(err), IsValidation: validationFailure(err)}
	var appErr *contracterrors.Error
	if errors.As(err, &appErr) {
		failure.Kind = appErr.Kind
		failure.Reason = appErr.Reason
		failure.Code = appErr.Code
	}
	return failure
}

func expectedType(t reflect.Type) string {
	if t == nil {
		// decoder 无法报告具体 Go 类型时可能出现 nil，这里保持用户可见消息语义通顺。
		return "正确"
	}
	for t.Kind() == reflect.Pointer {
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
