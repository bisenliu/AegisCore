package validation

import (
	"reflect"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

func registerEnumTranslation(trans ut.Translator) error {
	return trans.Add(RuleEnum, "{0}取值不合法", false)
}

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

func enumAllowedValues(value any) []string {
	if value == nil {
		return nil
	}
	field := reflect.ValueOf(value)
	if !field.IsValid() {
		return nil
	}
	if field.Kind() == reflect.Ptr {
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

func validateEnum(fl validator.FieldLevel) bool {
	value := fl.Field()
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	enum, ok := value.Interface().(Enum)
	return ok && enum.IsValid()
}
