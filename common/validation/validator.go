package validation

import (
	"fmt"

	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhtranslations "github.com/go-playground/validator/v10/translations/zh"
	"go.uber.org/zap"
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

func (v *Validator) Bind(c *gin.Context, dst any, binder Binder) error {
	if err := binder(c, dst); err != nil {
		return v.normalizeError(dst, err)
	}
	return v.Validate(dst)
}

func (v *Validator) BindOrAbort(c *gin.Context, dst any, binder Binder) bool {
	if err := v.Bind(c, dst, binder); err != nil {
		fields := []zap.Field{zap.Error(err), zap.String("path", c.Request.URL.Path)}
		if details := validationDetails(err); len(details) > 0 {
			fields = append(fields, zap.Any("errors", details))
		}
		logger.Error(c.Request.Context(), "invalid request", fields...)
		if validationFailure(err) {
			response.ValidationFailedWithErrors(c, publicMessage(err), validationDetails(err))
		} else {
			response.BadRequest(c, publicMessage(err))
		}
		c.Abort()
		return false
	}
	return true
}
