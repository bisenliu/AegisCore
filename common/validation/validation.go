package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhtranslations "github.com/go-playground/validator/v10/translations/zh"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	DefaultLocale       = "zh"
	ErrEmptyRequestBody = "请求体参数不能为空"
	ErrValidationFailed = "请求参数验证失败"
)

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

var Module = fx.Module("validation", fx.Provide(NewDefault))

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
	if err := validate.RegisterValidation("enum", validateEnum); err != nil {
		return nil, fmt.Errorf("register enum validation: %w", err)
	}
	if err := validate.RegisterTranslation("enum", trans, registerEnumTranslation, translateEnum); err != nil {
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

func URIBinder(c *gin.Context, dst any) error {
	values := make(url.Values, len(c.Params))
	for _, param := range c.Params {
		values.Set(param.Key, param.Value)
	}
	return bindValues(dst, values, "uri")
}

func QueryBinder(c *gin.Context, dst any) error {
	return bindValues(dst, c.Request.URL.Query(), "query", "form")
}

func JSONBinder(c *gin.Context, dst any) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return &Error{Message: ErrEmptyRequestBody, Code: response.CodeBadRequest}
	}
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func FormBinder(c *gin.Context, dst any) error {
	if err := c.Request.ParseForm(); err != nil {
		return err
	}
	values := c.Request.PostForm
	if c.Request.Method == http.MethodGet {
		values = c.Request.Form
	}
	return bindValues(dst, values, "form")
}

func bindValues(dst any, values url.Values, tags ...string) error {
	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return fmt.Errorf("validation bind target must be a non-nil pointer")
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return fmt.Errorf("validation bind target must point to a struct")
	}
	return bindStruct(value, values, tags)
}

func bindStruct(value reflect.Value, values url.Values, tags []string) error {
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		structField := typ.Field(i)
		if structField.PkgPath != "" && !structField.Anonymous {
			continue
		}
		if structField.Anonymous && field.Kind() == reflect.Struct {
			if err := bindStruct(field, values, tags); err != nil {
				return err
			}
			continue
		}
		name := bindingName(structField, tags)
		if name == "" {
			continue
		}
		rawValues, ok := values[name]
		if !ok || len(rawValues) == 0 {
			continue
		}
		if err := setField(field, rawValues); err != nil {
			return &bindFieldError{field: displayNameFromField(structField), typ: field.Type(), err: err}
		}
	}
	return nil
}

func bindingName(field reflect.StructField, tags []string) string {
	for _, tag := range tags {
		name := strings.SplitN(field.Tag.Get(tag), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return ""
}

func setField(field reflect.Value, rawValues []string) error {
	if !field.CanSet() {
		return nil
	}
	if field.Kind() == reflect.Ptr {
		if rawValues[0] == "" {
			return nil
		}
		field.Set(reflect.New(field.Type().Elem()))
		return setField(field.Elem(), rawValues)
	}
	if field.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(field.Type(), 0, len(rawValues))
		for _, raw := range rawValues {
			elem := reflect.New(field.Type().Elem()).Elem()
			if err := setScalar(elem, raw); err != nil {
				return err
			}
			slice = reflect.Append(slice, elem)
		}
		field.Set(slice)
		return nil
	}
	return setScalar(field, rawValues[0])
}

func setScalar(field reflect.Value, raw string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(value)
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(value)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}

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
		return &Error{Message: ErrValidationFailed, Fields: fields, Code: response.CodeValidationFailed}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := displayName(dst, typeErr.Field)
		return &Error{Message: fmt.Sprintf("%s字段类型不正确，应为%s类型", field, expectedType(typeErr.Type)), Code: response.CodeBadRequest}
	}
	var bindErr *bindFieldError
	if errors.As(err, &bindErr) {
		return &Error{Message: fmt.Sprintf("%s字段类型不正确，应为%s类型", bindErr.field, expectedType(bindErr.typ)), Code: response.CodeBadRequest}
	}
	if errors.Is(err, io.EOF) {
		return &Error{Message: ErrEmptyRequestBody, Code: response.CodeBadRequest}
	}
	return err
}

func registerEnumTranslation(trans ut.Translator) error {
	return trans.Add("enum", "{0}不合法", false)
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

func fieldName(fld reflect.StructField) string {
	if label := fld.Tag.Get("label"); label != "" {
		return label
	}
	for _, tag := range []string{"json", "form", "uri", "query"} {
		name := strings.SplitN(fld.Tag.Get(tag), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return fld.Name
}

func publicMessage(err error) string {
	var validationErr *Error
	if errors.As(err, &validationErr) && validationErr.Message != "" {
		return validationErr.Message
	}
	return err.Error()
}

func validationFailure(err error) bool {
	var validationErr *Error
	return errors.As(err, &validationErr) && validationErr.Code == response.CodeValidationFailed
}

func validationDetails(err error) []FieldError {
	var validationErr *Error
	if errors.As(err, &validationErr) {
		return validationErr.Fields
	}
	return nil
}

func expectedType(t reflect.Type) string {
	if t == nil {
		return "正确"
	}
	for t.Kind() == reflect.Ptr {
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

func displayName(dst any, path string) string {
	if path == "" {
		return "参数"
	}
	typ := reflect.TypeOf(dst)
	for typ != nil && typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ == nil || typ.Kind() != reflect.Struct {
		return path
	}
	return displayNameFromType(typ, strings.Split(path, "."))
}

func displayNameFromType(typ reflect.Type, parts []string) string {
	if len(parts) == 0 {
		return typ.Name()
	}
	part := parts[0]
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonName := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if jsonName != part && field.Name != part {
			continue
		}
		if len(parts) == 1 {
			return displayNameFromField(field)
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct {
			return displayNameFromType(fieldType, parts[1:])
		}
		return displayNameFromField(field)
	}
	return part
}

func displayNameFromField(field reflect.StructField) string {
	if label := field.Tag.Get("label"); label != "" {
		return label
	}
	if jsonName := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]; jsonName != "" && jsonName != "-" {
		return jsonName
	}
	return fieldName(field)
}

func validationFieldNames(dst any, fieldErr validator.FieldError) (string, string) {
	field, label, ok := validationFieldNamesFromPath(dst, fieldErr.StructNamespace())
	if ok {
		return field, label
	}
	name := fieldErr.Field()
	return name, name
}

func validationFieldNamesFromPath(dst any, namespace string) (string, string, bool) {
	parts := strings.Split(namespace, ".")
	if len(parts) < 2 {
		return "", "", false
	}
	typ := reflect.TypeOf(dst)
	for typ != nil && typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ == nil || typ.Kind() != reflect.Struct {
		return "", "", false
	}

	fieldParts := make([]string, 0, len(parts)-1)
	label := ""
	for _, part := range parts[1:] {
		part = strings.SplitN(part, "[", 2)[0]
		structField, ok := typ.FieldByName(part)
		if !ok {
			return "", "", false
		}
		name := requestFieldName(structField)
		if name == "" {
			return "", "", false
		}
		fieldParts = append(fieldParts, name)
		label = displayNameFromField(structField)

		typ = structField.Type
		for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			break
		}
	}
	if len(fieldParts) == 0 || label == "" {
		return "", "", false
	}
	return strings.Join(fieldParts, "."), label, true
}

func requestFieldName(field reflect.StructField) string {
	for _, tag := range []string{"json", "form", "uri", "query"} {
		name := strings.SplitN(field.Tag.Get(tag), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return field.Name
}
