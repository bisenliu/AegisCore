package ginvalidation

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/validation"
	"github.com/gin-gonic/gin"
)

// Binder 将 Gin context 中的请求数据绑定到 dst。
type Binder func(*gin.Context, any) error

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

// JSONBinder 将 JSON 请求体绑定到 dst，并允许未知字段。
func JSONBinder(c *gin.Context, dst any) error {
	return jsonBinder(c, dst, false)
}

// StrictJSONBinder 将 JSON 请求体绑定到 dst，并拒绝未知字段。
func StrictJSONBinder(c *gin.Context, dst any) error {
	return jsonBinder(c, dst, true)
}

// JSONBinderWithOptions 返回按未知字段策略配置的 JSON binder。
func JSONBinderWithOptions(disallowUnknownFields bool) Binder {
	return func(c *gin.Context, dst any) error {
		return jsonBinder(c, dst, disallowUnknownFields)
	}
}

func jsonBinder(c *gin.Context, dst any, disallowUnknownFields bool) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return &validation.Error{Message: validation.ErrEmptyRequestBody, Code: response.CodeBadRequest}
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
		return &validation.Error{Message: validation.ErrTrailingJSONBody, Code: response.CodeBadRequest}
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
