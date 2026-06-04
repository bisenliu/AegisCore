package validation

import (
	"reflect"

	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

const (
	// DefaultLocale 选择内置校验翻译语言。
	DefaultLocale = "zh"
	// ErrEmptyRequestBody 表示请求需要 JSON body 但内容为空。
	ErrEmptyRequestBody = "请求体参数不能为空"
	// ErrValidationFailed 是 validator 规则失败时的响应信封消息。
	ErrValidationFailed = "请求参数验证失败"
	// ErrTrailingJSONBody 表示请求体包含多个 JSON 值，必须拒绝解析。
	ErrTrailingJSONBody = "请求体只能包含一个 JSON 值"
	// RuleEnum 是枚举型请求字段使用的自定义 validator 标签。
	RuleEnum = "enum"
	// TagLabel 是用于校验错误展示名称的结构体标签。
	TagLabel = "label"
	// TagJSON 是用于提取 JSON 请求字段名的结构体标签。
	TagJSON = "json"
	// TagForm 是用于提取表单请求字段名的结构体标签。
	TagForm = "form"
	// TagURI 是用于提取 URI 参数字段名的结构体标签。
	TagURI = "uri"
	// TagQuery 是用于提取 query 参数字段名的结构体标签。
	TagQuery = "query"
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

type EnumValues interface {
	AllowedValues() []string
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
