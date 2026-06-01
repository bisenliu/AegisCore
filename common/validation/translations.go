package validation

import (
	"reflect"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

func registerEnumTranslation(trans ut.Translator) error {
	return trans.Add(RuleEnum, "{0}不合法", false)
}

func translateEnum(trans ut.Translator, fe validator.FieldError) string {
	msg, err := trans.T(fe.Tag(), fe.Field())
	if err != nil {
		return fe.Field() + "不合法"
	}
	return msg
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
