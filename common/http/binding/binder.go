package binding

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/validation"
)

// Binder 将 Gin context 中的请求数据绑定到 dst。
type Binder func(*gin.Context, any) error

// Compose 按顺序执行多个 binder，并由调用方统一执行校验。
func Compose(binders ...Binder) Binder {
	return func(c *gin.Context, dst any) error {
		for _, binder := range binders {
			if err := binder(c, dst); err != nil {
				return err
			}
		}
		return nil
	}
}

// URIBinder 使用 uri tag 将 Gin 路径参数绑定到 dst。
func URIBinder(c *gin.Context, dst any) error {
	values := make(url.Values, len(c.Params))
	for _, param := range c.Params {
		values.Set(param.Key, param.Value)
	}
	return validation.BindValues(dst, values, validation.TagURI)
}

// QueryBinder 使用 query 和 form tag 将 URL 查询参数绑定到 dst。
func QueryBinder(c *gin.Context, dst any) error {
	return validation.BindValues(dst, c.Request.URL.Query(), validation.TagQuery, validation.TagForm)
}

// HeaderBinder 使用 header tag 将 HTTP header 绑定到 dst。
func HeaderBinder(c *gin.Context, dst any) error {
	values := make(url.Values, len(c.Request.Header))
	for name, rawValues := range c.Request.Header {
		for _, value := range rawValues {
			values.Add(name, value)
		}
	}
	return validation.BindValues(dst, values, validation.TagHeader)
}

// JSONBinder 将 JSON 请求体绑定到 dst，并允许未知字段。
func JSONBinder(c *gin.Context, dst any) error {
	return jsonBinder(c, dst, false)
}

// StrictJSONBinder 将 JSON 请求体绑定到 dst，并拒绝未知字段。
func StrictJSONBinder(c *gin.Context, dst any) error {
	return jsonBinder(c, dst, true)
}

func jsonBinder(c *gin.Context, dst any, disallowUnknownFields bool) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return &validation.Error{Message: validation.ErrEmptyRequestBody, Kind: contracterrors.KindBadRequest, Reason: contracterrors.ReasonEmptyRequestBody, Code: contracterrors.CodeBadRequest}
	}
	decoder := json.NewDecoder(c.Request.Body)
	if disallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	// API 只接受一个 JSON 文档，避免拼接载荷隐藏尾随数据。
	if err := decoder.Decode(&extra); err != io.EOF {
		return &validation.Error{Message: validation.ErrTrailingJSONBody, Kind: contracterrors.KindBadRequest, Reason: contracterrors.ReasonTrailingJSONBody, Code: contracterrors.CodeBadRequest}
	}
	return nil
}

// FormBinder 使用 form tag 将表单值绑定到 dst。
func FormBinder(c *gin.Context, dst any) error {
	if err := c.Request.ParseForm(); err != nil {
		return err
	}
	values := c.Request.PostForm
	if c.Request.Method == http.MethodGet {
		// GET 请求的表单式输入来自查询字符串，ParseForm 会将其合并到 Form 中。
		values = c.Request.Form
	}
	return validation.BindValues(dst, values, validation.TagForm)
}
