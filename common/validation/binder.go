package validation

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
)

func URIBinder(c *gin.Context, dst any) error {
	values := make(url.Values, len(c.Params))
	for _, param := range c.Params {
		values.Set(param.Key, param.Value)
	}
	return bindValues(dst, values, TagURI)
}

func QueryBinder(c *gin.Context, dst any) error {
	return bindValues(dst, c.Request.URL.Query(), TagQuery, TagForm)
}

func JSONBinder(c *gin.Context, dst any) error {
	return jsonBinder(c, dst, false)
}

func StrictJSONBinder(c *gin.Context, dst any) error {
	return jsonBinder(c, dst, true)
}

func JSONBinderWithOptions(disallowUnknownFields bool) Binder {
	return func(c *gin.Context, dst any) error {
		return jsonBinder(c, dst, disallowUnknownFields)
	}
}

func jsonBinder(c *gin.Context, dst any, disallowUnknownFields bool) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return &Error{Message: ErrEmptyRequestBody, Code: response.CodeBadRequest}
	}
	decoder := json.NewDecoder(c.Request.Body)
	if disallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return &Error{Message: ErrTrailingJSONBody, Code: response.CodeBadRequest}
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
	return bindValues(dst, values, TagForm)
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
		if structField.Anonymous && field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if field.IsNil() && embeddedStructHasValues(field.Type().Elem(), values, tags) {
				field.Set(reflect.New(field.Type().Elem()))
			}
			if !field.IsNil() {
				if err := bindStruct(field.Elem(), values, tags); err != nil {
					return err
				}
			}
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

func embeddedStructHasValues(typ reflect.Type, values url.Values, tags []string) bool {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := bindingName(field, tags)
		if name != "" {
			if rawValues, ok := values[name]; ok && len(rawValues) > 0 {
				return true
			}
		}
		fieldType := field.Type
		if field.Anonymous {
			for fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct && embeddedStructHasValues(fieldType, values, tags) {
				return true
			}
		}
	}
	return false
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
	if field.CanAddr() {
		if unmarshaler, ok := field.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return unmarshaler.UnmarshalText([]byte(raw))
		}
	}
	if field.Type() == reflect.TypeOf(time.Duration(0)) {
		value, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		field.SetInt(int64(value))
		return nil
	}
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
