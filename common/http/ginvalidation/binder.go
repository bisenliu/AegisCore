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

type Binder func(*gin.Context, any) error

func URIBinder(c *gin.Context, dst any) error {
	values := make(url.Values, len(c.Params))
	for _, param := range c.Params {
		values.Set(param.Key, param.Value)
	}
	return validation.BindValues(dst, values, validation.TagURI)
}

func QueryBinder(c *gin.Context, dst any) error {
	return validation.BindValues(dst, c.Request.URL.Query(), validation.TagQuery, validation.TagForm)
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
	if err := decoder.Decode(&extra); err != io.EOF {
		return &validation.Error{Message: validation.ErrTrailingJSONBody, Code: response.CodeBadRequest}
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
	return validation.BindValues(dst, values, validation.TagForm)
}
