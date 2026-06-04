package validation

import (
	"fmt"

	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhtranslations "github.com/go-playground/validator/v10/translations/zh"
)

func NewDefault() (*Validator, error) {
	return New(Options{Locale: DefaultLocale})
}

func New(opts Options) (*Validator, error) {
	locale := opts.Locale
	if locale == "" {
		locale = DefaultLocale
	}
	if locale != DefaultLocale {
		return nil, fmt.Errorf("unsupported validation locale %q", locale)
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(fieldName)

	zhLocale := zh.New()
	uni := ut.New(zhLocale, zhLocale)
	trans, ok := uni.GetTranslator(locale)
	if !ok {
		return nil, fmt.Errorf("validation translator %q is unavailable", locale)
	}
	if err := zhtranslations.RegisterDefaultTranslations(validate, trans); err != nil {
		return nil, fmt.Errorf("register validation translations: %w", err)
	}
	if err := validate.RegisterValidation(RuleEnum, validateEnum); err != nil {
		return nil, fmt.Errorf("register enum validation: %w", err)
	}
	if err := validate.RegisterTranslation(RuleEnum, trans, registerEnumTranslation, translateEnum); err != nil {
		return nil, fmt.Errorf("register enum translation: %w", err)
	}

	return &Validator{validate: validate, trans: trans}, nil
}

func (v *Validator) Validate(dst any) error {
	if d, ok := dst.(Defaultable); ok {
		d.SetDefaults()
	}
	if err := v.validate.Struct(dst); err != nil {
		return v.normalizeError(dst, err)
	}
	if custom, ok := dst.(Validatable); ok {
		if err := custom.Validate(); err != nil {
			return v.normalizeError(dst, err)
		}
	}
	return nil
}

func (v *Validator) NormalizeError(dst any, err error) error {
	return v.normalizeError(dst, err)
}
