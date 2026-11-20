package validation

import (
	"reflect"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

// registerEnumTranslation 注册固定选项检查失败时的中文提示模板。
func registerEnumTranslation(trans ut.Translator) error {
	return trans.Add(RuleEnum, "{0}取值不合法", false)
}

// translateEnum 生成友好的选项错误提示，向用户列出允许选择的有效选项清单。
func translateEnum(trans ut.Translator, fe validator.FieldError) string {
	field := fe.Field()
	if values := enumAllowedValues(fe.Value()); len(values) > 0 {
		return field + "取值不合法，允许值为：" + strings.Join(values, "、")
	}
	msg, err := trans.T(fe.Tag(), fe.Field())
	if err != nil {
		return field + "取值不合法"
	}
	return msg
}

// enumAllowedValues 提取某个选项字段所有允许填写的备选值列表，用于展示给用户看。
func enumAllowedValues(value any) []string {
	if value == nil {
		return nil
	}
	field := reflect.ValueOf(value)
	if !field.IsValid() {
		return nil
	}
	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return nil
		}
		if values, ok := field.Interface().(EnumValues); ok {
			return values.AllowedValues()
		}
		field = field.Elem()
	}
	if values, ok := field.Interface().(EnumValues); ok {
		return values.AllowedValues()
	}
	return nil
}

// validateEnum 检查用户填写的值是否属于系统预设的有效选项之一。
func validateEnum(fl validator.FieldLevel) bool {
	value := fl.Field()
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	enum, ok := value.Interface().(Enum)
	return ok && enum.IsValid()
}
