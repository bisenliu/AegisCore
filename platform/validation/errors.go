package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/go-playground/validator/v10"

	contracterrors "github.com/aegiscore/platform/contract/errors"
)

// normalizeError 将 validator、JSON decoder 和字段 binder 的错误归一化为稳定的公开错误契约。
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

// publicMessage 返回适合直接展示给用户看的错误提示语，隐藏技术底层细节。
func publicMessage(err error) string {
	var validationErr *Error
	if errors.As(err, &validationErr) && validationErr.Message != "" {
		return validationErr.Message
	}
	return err.Error()
}

// validationFailure 检查错误是否属于用户输入内容未通过规则检查（如密码太短、邮箱格式不对等）。
func validationFailure(err error) bool {
	var appErr *contracterrors.Error
	return errors.As(err, &appErr) && appErr.Kind == contracterrors.KindValidation
}

// validationDetails 汇总所有不合格输入项的详细信息，便于在返回结果中逐一标明哪个输入框填写有误。
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

// expectedType 获取该字段期望输入的数据类型名称（如数字或文本），用于在用户填错类型时给出明确提示。
func expectedType(t reflect.Type) string {
	if t == nil {
		// 当无法确定具体期望类型时返回通用的“正确”二字，确保拼接出来的错误提示语句（如“请输入正确类型”）通顺自然。
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
