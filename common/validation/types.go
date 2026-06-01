package validation

import (
	"reflect"

	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

const (
	DefaultLocale       = "zh"
	ErrEmptyRequestBody = "请求体参数不能为空"
	ErrValidationFailed = "请求参数验证失败"
	ErrTrailingJSONBody = "请求体只能包含一个 JSON 值"
	RuleEnum            = "enum"
	TagLabel            = "label"
	TagJSON             = "json"
	TagForm             = "form"
	TagURI              = "uri"
	TagQuery            = "query"
)

var requestTags = []string{TagJSON, TagForm, TagURI, TagQuery}

type Binder func(*gin.Context, any) error

type Defaultable interface {
	SetDefaults()
}

type Validatable interface {
	Validate() error
}

type Enum interface {
	IsValid() bool
}

type FieldError struct {
	Field   string `json:"field"`
	Label   string `json:"label"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type Error struct {
	Message string        `json:"message"`
	Fields  []FieldError  `json:"fields,omitempty"`
	Code    response.Code `json:"-"`
}

type bindFieldError struct {
	field string
	typ   reflect.Type
	err   error
}

func (e *bindFieldError) Error() string {
	return e.err.Error()
}

func (e *bindFieldError) Unwrap() error {
	return e.err
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type Options struct {
	Locale string
}

type Validator struct {
	validate *validator.Validate
	trans    ut.Translator
}
