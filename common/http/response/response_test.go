package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
)

func TestFailureResponseErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("nil failure returns internal envelope", func(t *testing.T) {
		ctx, recorder := newTestContext()

		Fail(ctx, nil)

		var envelope contractresponse.Envelope
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		require.Equal(t, http.StatusInternalServerError, recorder.Code)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeInternalError, envelope.Code)
		require.Equal(t, contractresponse.MessageInternalError, envelope.Message)
	})

	t.Run("ordinary failure omits errors", func(t *testing.T) {
		ctx, recorder := newTestContext()

		BadRequest(ctx, "请求格式错误")

		var envelope contractresponse.Envelope
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeBadRequest, envelope.Code)
		require.Equal(t, "请求格式错误", envelope.Message)
		require.Nil(t, envelope.Errors)
	})

	t.Run("token invalid failure", func(t *testing.T) {
		ctx, recorder := newTestContext()

		TokenInvalid(ctx, contractresponse.MessageAuthInvalid)

		var envelope contractresponse.Envelope
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeTokenInvalid, envelope.Code)
		require.Equal(t, contractresponse.MessageAuthInvalid, envelope.Message)
	})

	t.Run("token expired failure", func(t *testing.T) {
		ctx, recorder := newTestContext()

		TokenExpired(ctx, contractresponse.MessageAuthInvalid)

		var envelope contractresponse.Envelope
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.False(t, envelope.Success)
		require.Equal(t, contracterrors.CodeTokenExpired, envelope.Code)
		require.Equal(t, contractresponse.MessageAuthInvalid, envelope.Message)
	})

	t.Run("validation failure includes errors", func(t *testing.T) {
		ctx, recorder := newTestContext()
		details := []map[string]string{{"field": "email", "label": "邮箱", "rule": "email", "message": "邮箱格式不正确"}}

		ValidationFailedWithErrors(ctx, "请求参数验证失败", details)

		var body map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Equal(t, false, body["success"])
		require.Equal(t, float64(contracterrors.CodeValidationFailed), body["code"])
		require.Equal(t, "请求参数验证失败", body["message"])
		require.NotContains(t, body, "data")
		errors, ok := body["errors"].([]any)
		require.True(t, ok)
		require.Len(t, errors, 1)
		field, ok := errors[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "email", field["field"])
		require.Equal(t, "邮箱", field["label"])
		require.Equal(t, "email", field["rule"])
		require.Equal(t, "邮箱格式不正确", field["message"])
	})
}

func TestFailureResponseAnnotatesSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("internal application error records sanitized event", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)

		WriteError(ctx, contracterrors.InternalError(errors.New("database password token stacktrace leaked")))

		require.Equal(t, http.StatusInternalServerError, recorder.Code)
		ended := endSpan(t, span)
		require.Equal(t, codes.Error, ended.Status().Code)
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeInternalError))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusInternalServerError)
		event := findSpanEvent(t, ended, "exception")
		assertSpanEventStringAttribute(t, event, spanAttrErrorType, spanErrorTypeApplication)
		assertNoSensitiveSpanEventText(t, event, "database", "password", "token", "stacktrace")
	})

	t.Run("client application error only sets low cardinality attributes", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)

		BadRequest(ctx, "请求格式错误 password token")

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		ended := endSpan(t, span)
		require.Equal(t, codes.Unset, ended.Status().Code)
		require.Empty(t, ended.Events())
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeBadRequest))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusBadRequest)
	})

	t.Run("fail maps unknown error to internal span error", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)

		Fail(ctx, errors.New("sql args password token"))

		require.Equal(t, http.StatusInternalServerError, recorder.Code)
		ended := endSpan(t, span)
		require.Equal(t, codes.Error, ended.Status().Code)
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeInternalError))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusInternalServerError)
		event := findSpanEvent(t, ended, "exception")
		assertNoSensitiveSpanEventText(t, event, "sql", "password", "token")
	})

	t.Run("validation error does not leak field details to span", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)
		details := []map[string]string{{"field": "password", "label": "密码", "rule": "required", "message": "token Authorization Cookie"}}

		ValidationFailedWithErrors(ctx, "请求参数验证失败", details)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		ended := endSpan(t, span)
		require.Equal(t, codes.Unset, ended.Status().Code)
		require.Empty(t, ended.Events())
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeValidationFailed))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusBadRequest)
		for _, attr := range ended.Attributes() {
			text := attr.Value.String()
			require.NotContains(t, text, "password")
			require.NotContains(t, text, "token")
			require.NotContains(t, text, "Authorization")
			require.NotContains(t, text, "Cookie")
		}
	})
}

func TestOKWithPaginatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	OK(ctx, map[string]any{"id": 1, "name": "Alice"})

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, true, body["success"])
	require.Equal(t, float64(contracterrors.CodeOK), body["code"])
	require.Equal(t, contractresponse.MessageOK, body["message"])
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(1), data["id"])
	require.Equal(t, "Alice", data["name"])
}

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return ctx, recorder
}

func newTestContextWithSpan(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, sdktrace.ReadWriteSpan) {
	t.Helper()
	ctx, recorder := newTestContext()
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	tracer := provider.Tracer("common-http-response-test")
	requestCtx, span := tracer.Start(ctx.Request.Context(), "response")
	ctx.Request = ctx.Request.WithContext(requestCtx)
	t.Cleanup(func() {
		if span.IsRecording() {
			span.End()
		}
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return ctx, recorder, spanRecorder.Started()[0]
}

func endSpan(t *testing.T, span sdktrace.ReadWriteSpan) sdktrace.ReadOnlySpan {
	t.Helper()
	span.End()
	return span
}

func assertSpanIntAttribute(t *testing.T, span sdktrace.ReadOnlySpan, key string, want int) {
	t.Helper()
	var got *int
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			value := int(attr.Value.AsInt64())
			got = &value
			break
		}
	}
	require.NotNil(t, got, "span attribute %s missing in %#v", key, span.Attributes())
	require.Equal(t, want, *got)
}

func findSpanEvent(t *testing.T, span sdktrace.ReadOnlySpan, name string) sdktrace.Event {
	t.Helper()
	var found sdktrace.Event
	foundEvent := false
	for _, event := range span.Events() {
		if event.Name == name {
			found = event
			foundEvent = true
			break
		}
	}
	require.True(t, foundEvent, "span event %q missing in %#v", name, span.Events())
	return found
}

func assertSpanEventStringAttribute(t *testing.T, event sdktrace.Event, key string, want string) {
	t.Helper()
	var got *string
	for _, attr := range event.Attributes {
		if string(attr.Key) == key {
			value := attr.Value.AsString()
			got = &value
			break
		}
	}
	require.NotNil(t, got, "span event attribute %s missing in %#v", key, event.Attributes)
	require.Equal(t, want, *got)
}

func assertNoSensitiveSpanEventText(t *testing.T, event sdktrace.Event, forbidden ...string) {
	t.Helper()
	for _, attr := range event.Attributes {
		text := attr.Value.String()
		for _, item := range forbidden {
			require.NotContains(t, text, item)
		}
	}
}
