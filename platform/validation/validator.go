package validation

import (
	"fmt"

	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhtranslations "github.com/go-playground/validator/v10/translations/zh"
)

// NewDefault 使用包默认 locale 创建 Validator。
func NewDefault() (*Validator, error) {
	return New(Options{Locale: DefaultLocale})
}

// New 创建已注册翻译和自定义枚举校验的 Validator。
func New(opts Options) (*Validator, error) {
	locale := opts.Locale
	if locale == "" {
		locale = DefaultLocale
	}
	if locale != DefaultLocale {
		// 当前只注册 zh 翻译，提前失败可避免生成中英混杂的错误消息。
		return nil, fmt.Errorf("unsupported validation locale %q", locale)
	}

	// RequiredStructEnabled 让 request DTO 嵌套结构体上的 required tag 行为保持一致。
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

// Validate 先应用默认值，再执行 struct tag 校验，最后执行请求自定义校验。
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

// NormalizeError 将 bind、JSON 和 validator 错误转换为 AegisCore 校验错误。
func (v *Validator) NormalizeError(dst any, err error) error {
	return v.normalizeError(dst, err)
}
