package validation

import (
	"reflect"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"

	contracterrors "github.com/aegiscore/platform/contract/errors"
)

const (
	// DefaultLocale 选择内置校验翻译语言。
	DefaultLocale = "zh"
	// ErrEmptyRequestBody 表示请求需要 JSON body 但内容为空。
	ErrEmptyRequestBody = "请求体参数不能为空"
	// ErrValidationFailed 是校验规则未通过时的标准对外提示消息。
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
	// TagHeader 是用于提取 HTTP header 请求字段名的结构体标签。
	TagHeader = "header"
	// TagURI 是用于提取 URI 参数字段名的结构体标签。
	TagURI = "uri"
	// TagQuery 是用于提取 query 参数字段名的结构体标签。
	TagQuery = "query"
)

// Defaultable 标记可在校验前填充安全默认值的请求值。
type Defaultable interface {
	SetDefaults()
}

// Validatable 标记可在 struct tag 校验后执行自定义校验的请求值。
type Validatable interface {
	Validate() error
}

// Enum 由可针对固定枚举域自校验的值实现。
type Enum interface {
	IsValid() bool
}

// EnumValues 暴露允许的枚举值，用于校验错误消息。
type EnumValues interface {
	AllowedValues() []string
}

// FieldError 描述返回给 API 客户端的单个请求字段校验失败。
type FieldError struct {
	Field   string `json:"field"`
	Label   string `json:"label"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Error 是规范化后的校验或绑定错误，可携带字段明细。
type Error struct {
	Message string                `json:"message"`
	Fields  []FieldError          `json:"fields,omitempty"`
	Kind    contracterrors.Kind   `json:"-"`
	Reason  contracterrors.Reason `json:"-"`
	Code    contracterrors.Code   `json:"-"`
}

// Failure 包含从校验或绑定错误派生出的响应元数据。
type Failure struct {
	Message      string
	Fields       []FieldError
	Kind         contracterrors.Kind
	Reason       contracterrors.Reason
	Code         contracterrors.Code
	IsValidation bool
}

type bindFieldError struct {
	field string
	typ   reflect.Type
	err   error
}

// Options 配置 Validator 构造行为。
type Options struct {
	Locale string
}

// Validator 用 AegisCore 翻译和规范化规则包装 go-playground validator。
type Validator struct {
	validate *validator.Validate
	trans    ut.Translator
}

// Error 返回字段绑定失败的底层错误消息。
func (e *bindFieldError) Error() string {
	return e.err.Error()
}

// Unwrap 暴露底层绑定错误，供 errors.Is 和 errors.As 继续匹配。
func (e *bindFieldError) Unwrap() error {
	return e.err
}

// Error 返回规范化公开消息，并允许 nil receiver。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Unwrap 返回该校验错误对应的标准业务错误对象，支持标准库 errors.As 识别和处理。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return validationAppError(e)
}

// validationAppError 将表单输入检查失败整理成对外的标准业务错误，同时附带具体每个输入项的错误原因。
func validationAppError(e *Error) *contracterrors.Error {
	kind := e.Kind
	reason := e.Reason
	code := e.Code
	if kind == "" {
		if len(e.Fields) > 0 {
			kind = contracterrors.KindValidation
		} else {
			kind = contracterrors.KindBadRequest
		}
	}
	if reason == "" {
		reason = defaultReason(kind)
	}
	if code == 0 {
		code = defaultCode(kind)
	}
	return contracterrors.New(kind, reason, code, e.Message)
}

// defaultReason 根据错误的类别（如输入不合法、数据冲突等），给出默认的技术原因标识。
func defaultReason(kind contracterrors.Kind) contracterrors.Reason {
	switch kind {
	case contracterrors.KindValidation:
		return contracterrors.ReasonValidationFailed
	case contracterrors.KindBadRequest:
		return contracterrors.ReasonBadRequest
	default:
		return contracterrors.ReasonInternalError
	}
}

// defaultCode 根据错误的类别，给出统一的数字或文本错误代号。
func defaultCode(kind contracterrors.Kind) contracterrors.Code {
	switch kind {
	case contracterrors.KindValidation:
		return contracterrors.CodeValidationFailed
	case contracterrors.KindBadRequest:
		return contracterrors.CodeBadRequest
	default:
		return contracterrors.CodeInternalError
	}
}
