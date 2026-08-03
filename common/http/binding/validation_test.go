package binding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/validation"
)

func TestJSONBinder(t *testing.T) {
	validator := newTestValidator(t)

	type request struct {
		ID int64 `json:"id" validate:"required,gt=0" label:"用户ID"`
	}

	t.Run("valid body", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123}`)
		var req request
		require.NoError(t, Bind(validator, ctx, &req, JSONBinder))
		require.Equal(t, int64(123), req.ID)
	})

	t.Run("empty body", func(t *testing.T) {
		ctx := newJSONContext("")
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var validationErr *validation.Error
		require.ErrorAs(t, err, &validationErr)
		require.Equal(t, validation.ErrEmptyRequestBody, validationErr.Message)
		require.Equal(t, contracterrors.KindBadRequest, validationErr.Kind)
		require.Equal(t, contracterrors.ReasonEmptyRequestBody, validationErr.Reason)
		require.Equal(t, contracterrors.CodeBadRequest, validationErr.Code)
	})

	t.Run("type mismatch", func(t *testing.T) {
		ctx := newJSONContext(`{"id":"bad"}`)
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var validationErr *validation.Error
		require.ErrorAs(t, err, &validationErr)
		require.Equal(t, "用户ID字段类型不正确，应为整数类型", validationErr.Message)
		require.Equal(t, contracterrors.KindBadRequest, validationErr.Kind)
		require.Equal(t, contracterrors.ReasonRequestBindingFailed, validationErr.Reason)
		require.Equal(t, contracterrors.CodeBadRequest, validationErr.Code)
	})

	t.Run("trailing body", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123} {"id":456}`)
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var validationErr *validation.Error
		require.ErrorAs(t, err, &validationErr)
		require.Equal(t, validation.ErrTrailingJSONBody, validationErr.Message)
		require.Equal(t, contracterrors.KindBadRequest, validationErr.Kind)
		require.Equal(t, contracterrors.ReasonTrailingJSONBody, validationErr.Reason)
		require.Equal(t, contracterrors.CodeBadRequest, validationErr.Code)
	})

	t.Run("unknown field compatible by default", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123,"extra":true}`)
		var req request
		require.NoError(t, Bind(validator, ctx, &req, JSONBinder))
	})

	t.Run("unknown field rejected in strict mode", func(t *testing.T) {
		ctx := newJSONContext(`{"id":123,"extra":true}`)
		var req request
		require.Error(t, Bind(validator, ctx, &req, StrictJSONBinder))
	})

	t.Run("limited body normalizes max bytes error", func(t *testing.T) {
		ctx, recorder := newJSONContextWithRecorder(`{"id":123}`)
		ctx.Request.Body = http.MaxBytesReader(recorder, ctx.Request.Body, 8)
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var appErr *contracterrors.Error
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, contracterrors.KindPayloadTooLarge, appErr.Kind)
		require.Equal(t, contracterrors.ReasonRequestBodyTooLarge, appErr.Reason)
		require.Equal(t, contracterrors.CodeRequestBodyTooLarge, appErr.Code)
	})

	t.Run("trailing data normalizes max bytes error", func(t *testing.T) {
		ctx, recorder := newJSONContextWithRecorder(`{"id":1} {"id":456}`)
		ctx.Request.Body = http.MaxBytesReader(recorder, ctx.Request.Body, 8)
		var req request
		err := Bind(validator, ctx, &req, JSONBinder)
		var appErr *contracterrors.Error
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, contracterrors.KindPayloadTooLarge, appErr.Kind)
		require.Equal(t, contracterrors.ReasonRequestBodyTooLarge, appErr.Reason)
		require.Equal(t, contracterrors.CodeRequestBodyTooLarge, appErr.Code)
	})
}

func TestBinders(t *testing.T) {
	validator := newTestValidator(t)

	t.Run("uri", func(t *testing.T) {
		type request struct {
			ID int64 `uri:"id" validate:"required,gt=0"`
		}
		ctx := newRequestContext(http.MethodGet, "/users/123", "")
		ctx.Params = gin.Params{{Key: "id", Value: "123"}}
		var req request
		require.NoError(t, Bind(validator, ctx, &req, URIBinder))
		require.Equal(t, int64(123), req.ID)
	})

	t.Run("uri type mismatch", func(t *testing.T) {
		type request struct {
			ID int64 `uri:"id" validate:"required,gt=0" label:"用户ID"`
		}
		ctx := newRequestContext(http.MethodGet, "/users/bad", "")
		ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
		var req request
		err := Bind(validator, ctx, &req, URIBinder)
		var validationErr *validation.Error
		require.ErrorAs(t, err, &validationErr)
		require.Equal(t, "用户ID字段类型不正确，应为整数类型", validationErr.Message)
		require.Equal(t, contracterrors.KindBadRequest, validationErr.Kind)
		require.Equal(t, contracterrors.ReasonRequestBindingFailed, validationErr.Reason)
		require.Equal(t, contracterrors.CodeBadRequest, validationErr.Code)
	})

	t.Run("query", func(t *testing.T) {
		type request struct {
			Page int `query:"page" validate:"required,gt=0"`
		}
		ctx := newRequestContext(http.MethodGet, "/users?page=2", "")
		var req request
		require.NoError(t, Bind(validator, ctx, &req, QueryBinder))
		require.Equal(t, 2, req.Page)
	})

	t.Run("form", func(t *testing.T) {
		type request struct {
			Name string `form:"name" validate:"required"`
		}
		ctx := newRequestContext(http.MethodPost, "/users", "name=aegis")
		ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var req request
		require.NoError(t, Bind(validator, ctx, &req, FormBinder))
		require.Equal(t, "aegis", req.Name)
	})

	t.Run("custom text unmarshaler and duration", func(t *testing.T) {
		type request struct {
			Status testTextValue `query:"status"`
			TTL    time.Duration `query:"ttl"`
		}
		ctx := newRequestContext(http.MethodGet, "/users?status=active&ttl=5s", "")
		var req request
		require.NoError(t, Bind(validator, ctx, &req, QueryBinder))
		require.Equal(t, testTextValue("parsed:active"), req.Status)
		require.Equal(t, 5*time.Second, req.TTL)
	})

	t.Run("embedded pointer struct", func(t *testing.T) {
		type request struct {
			*TestEmbedded
		}
		ctx := newRequestContext(http.MethodGet, "/users?page=3", "")
		var req request
		require.NoError(t, Bind(validator, ctx, &req, QueryBinder))
		require.NotNil(t, req.TestEmbedded)
		require.Equal(t, 3, req.Page)
	})

	t.Run("header", func(t *testing.T) {
		type request struct {
			Token string `header:"Authorization" validate:"required"`
		}
		ctx := newRequestContext(http.MethodGet, "/users", "")
		ctx.Request.Header.Set("Authorization", "Bearer token")
		var req request
		require.NoError(t, Bind(validator, ctx, &req, HeaderBinder))
		require.Equal(t, "Bearer token", req.Token)
	})

	t.Run("compose", func(t *testing.T) {
		type request struct {
			ID   int64  `uri:"id" validate:"required,gt=0"`
			Name string `json:"name" validate:"required"`
		}
		ctx := newRequestContext(http.MethodPost, "/users/123", `{"name":"aegis"}`)
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "id", Value: "123"}}
		var req request
		require.NoError(t, Bind(validator, ctx, &req, Compose(URIBinder, JSONBinder)))
		require.Equal(t, int64(123), req.ID)
		require.Equal(t, "aegis", req.Name)
	})

	t.Run("compose returns first error", func(t *testing.T) {
		type request struct {
			ID   int64  `uri:"id" validate:"required,gt=0" label:"用户ID"`
			Name string `json:"name" validate:"required"`
		}
		ctx := newRequestContext(http.MethodPost, "/users/bad", `{"name":"aegis"}`)
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
		var req request
		err := Bind(validator, ctx, &req, Compose(URIBinder, JSONBinder))
		var validationErr *validation.Error
		require.ErrorAs(t, err, &validationErr)
		require.Equal(t, "用户ID字段类型不正确，应为整数类型", validationErr.Message)
		require.Empty(t, req.Name)
	})
}

func TestBindOrAbort(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		Name string `json:"name" validate:"required"`
	}
	ctx, recorder := newJSONContextWithRecorder(`{}`)
	ctx.Request = ctx.Request.WithContext(logger.WithRequestID(ctx.Request.Context(), "request-123"))
	logs := captureValidationLogs(t, ctx)
	var req request
	require.False(t, BindOrAbort(validator, ctx, &req, JSONBinder))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var envelope response.Envelope
	require.NoError(t, jsonUnmarshal(recorder.Body.String(), &envelope))
	require.False(t, envelope.Success)
	require.Equal(t, contracterrors.CodeValidationFailed, envelope.Code)
	require.Equal(t, validation.ErrValidationFailed, envelope.Message)
	envelopeErrors, ok := envelope.Errors.([]any)
	require.True(t, ok)
	require.Len(t, envelopeErrors, 1)
	entries := logs.All()
	require.Len(t, entries, 1)
	entry := entries[0]
	require.Equal(t, zapcore.WarnLevel, entry.Level)
	require.Equal(t, "invalid request", entry.Message)
	fields := entry.ContextMap()
	require.Equal(t, "__unmatched__", fields["path"])
	require.Equal(t, "request-123", fields[logger.RequestIDField])
	require.NotNil(t, fields["error"])
	errorsField, ok := fields["errors"].([]validation.FieldError)
	require.True(t, ok)
	require.Len(t, errorsField, 1)
	require.Equal(t, "name", errorsField[0].Field)
	require.Equal(t, "required", errorsField[0].Rule)
}

func TestBindOrAbortTypeMismatchUsesBadRequest(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		ID int64 `uri:"id" label:"用户ID"`
	}
	ctx, recorder := newRequestContextWithRecorder(http.MethodGet, "/users/bad", "")
	logs := captureValidationLogs(t, ctx)
	ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
	var req request
	require.False(t, BindOrAbort(validator, ctx, &req, URIBinder))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var envelope response.Envelope
	require.NoError(t, jsonUnmarshal(recorder.Body.String(), &envelope))
	require.False(t, envelope.Success)
	require.Equal(t, contracterrors.CodeBadRequest, envelope.Code)
	require.Equal(t, "用户ID字段类型不正确，应为整数类型", envelope.Message)
	require.Nil(t, envelope.Errors)
	entries := logs.All()
	require.Len(t, entries, 1)
	entry := entries[0]
	require.Equal(t, zapcore.WarnLevel, entry.Level)
	require.Equal(t, "invalid request", entry.Message)
	fields := entry.ContextMap()
	require.Equal(t, "__unmatched__", fields["path"])
	require.NotNil(t, fields["error"])
	require.NotContains(t, fields, "errors")
}

func TestBindOrAbortLogsRouteTemplateForDynamicPath(t *testing.T) {
	validator := newTestValidator(t)
	type request struct {
		ID int64 `uri:"id" label:"用户ID"`
	}
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.DebugLevel)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(logger.ToContext(c.Request.Context(), zap.New(core)))
		c.Next()
	})
	engine.GET("/users/:id", func(c *gin.Context) {
		var req request
		require.False(t, BindOrAbort(validator, c, &req, URIBinder))
	})

	rawPath := "/users/bad"
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, rawPath, nil))

	entries := logs.All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "/users/:id", fields["path"])
	require.NotEqual(t, rawPath, fields["path"])
}

type testTextValue string

func (v *testTextValue) UnmarshalText(text []byte) error {
	*v = testTextValue("parsed:" + string(text))
	return nil
}

type TestEmbedded struct {
	Page int `query:"page"`
}

func newTestValidator(t *testing.T) *validation.Validator {
	t.Helper()
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	return validator
}

func newJSONContext(body string) *gin.Context {
	ctx, _ := newJSONContextWithRecorder(body)
	return ctx
}

func newJSONContextWithRecorder(body string) (*gin.Context, *httptest.ResponseRecorder) {
	ctx, recorder := newRequestContextWithRecorder(http.MethodPost, "/", body)
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func newRequestContext(method, target, body string) *gin.Context {
	ctx, _ := newRequestContextWithRecorder(method, target, body)
	return ctx
}

func newRequestContextWithRecorder(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.ContentLength = int64(len(body))
	ctx.Request = request
	return ctx, recorder
}

func jsonUnmarshal(raw string, dst any) error {
	return json.NewDecoder(strings.NewReader(raw)).Decode(dst)
}

func captureValidationLogs(t *testing.T, ctx *gin.Context) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	ctx.Request = ctx.Request.WithContext(logger.ToContext(ctx.Request.Context(), zap.New(core)))
	return logs
}
