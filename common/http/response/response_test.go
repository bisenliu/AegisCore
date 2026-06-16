package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusInternalServerError || envelope.Success || envelope.Code != contracterrors.CodeInternalError || envelope.Message != contractresponse.MessageInternalError {
			t.Fatalf("response = status %d envelope %#v", recorder.Code, envelope)
		}
	})

	t.Run("ordinary failure omits errors", func(t *testing.T) {
		ctx, recorder := newTestContext()

		BadRequest(ctx, "请求格式错误")

		var envelope contractresponse.Envelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusBadRequest || envelope.Success || envelope.Code != contracterrors.CodeBadRequest || envelope.Message != "请求格式错误" {
			t.Fatalf("response = status %d envelope %#v", recorder.Code, envelope)
		}
		if envelope.Errors != nil {
			t.Fatalf("errors = %#v, want nil", envelope.Errors)
		}
	})

	t.Run("token invalid failure", func(t *testing.T) {
		ctx, recorder := newTestContext()

		TokenInvalid(ctx, contractresponse.MessageAuthInvalid)

		var envelope contractresponse.Envelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusUnauthorized || envelope.Success || envelope.Code != contracterrors.CodeTokenInvalid || envelope.Message != contractresponse.MessageAuthInvalid {
			t.Fatalf("response = status %d envelope %#v", recorder.Code, envelope)
		}
	})

	t.Run("token expired failure", func(t *testing.T) {
		ctx, recorder := newTestContext()

		TokenExpired(ctx, contractresponse.MessageAuthInvalid)

		var envelope contractresponse.Envelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusUnauthorized || envelope.Success || envelope.Code != contracterrors.CodeTokenExpired || envelope.Message != contractresponse.MessageAuthInvalid {
			t.Fatalf("response = status %d envelope %#v", recorder.Code, envelope)
		}
	})

	t.Run("validation failure includes errors", func(t *testing.T) {
		ctx, recorder := newTestContext()
		details := []map[string]string{{"field": "email", "label": "邮箱", "rule": "email", "message": "邮箱格式不正确"}}

		ValidationFailedWithErrors(ctx, "请求参数验证失败", details)

		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusBadRequest || body["success"] != false || body["code"] != float64(contracterrors.CodeValidationFailed) || body["message"] != "请求参数验证失败" {
			t.Fatalf("response = status %d body %#v", recorder.Code, body)
		}
		if _, ok := body["data"]; ok {
			t.Fatalf("data = %#v, want omitted", body["data"])
		}
		errors, ok := body["errors"].([]any)
		if !ok || len(errors) != 1 {
			t.Fatalf("errors = %#v, want one error", body["errors"])
		}
		field, ok := errors[0].(map[string]any)
		if !ok || field["field"] != "email" || field["label"] != "邮箱" || field["rule"] != "email" || field["message"] != "邮箱格式不正确" {
			t.Fatalf("field error = %#v", errors[0])
		}
	})
}

func TestFailureResponseAnnotatesSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("internal application error records sanitized event", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)

		WriteError(ctx, contracterrors.InternalError(errors.New("database password token stacktrace leaked")))

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		ended := endSpan(t, span)
		if got := ended.Status().Code; got != codes.Error {
			t.Fatalf("span status = %s, want Error", got)
		}
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeInternalError))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusInternalServerError)
		event := findSpanEvent(t, ended, "exception")
		assertSpanEventStringAttribute(t, event, spanAttrErrorType, spanErrorTypeApplication)
		assertNoSensitiveSpanEventText(t, event, "database", "password", "token", "stacktrace")
	})

	t.Run("client application error only sets low cardinality attributes", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)

		BadRequest(ctx, "请求格式错误 password token")

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", recorder.Code)
		}
		ended := endSpan(t, span)
		if got := ended.Status().Code; got != codes.Unset {
			t.Fatalf("span status = %s, want Unset", got)
		}
		if len(ended.Events()) != 0 {
			t.Fatalf("span events = %#v, want none", ended.Events())
		}
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeBadRequest))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusBadRequest)
	})

	t.Run("fail maps unknown error to internal span error", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)

		Fail(ctx, errors.New("sql args password token"))

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		ended := endSpan(t, span)
		if got := ended.Status().Code; got != codes.Error {
			t.Fatalf("span status = %s, want Error", got)
		}
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeInternalError))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusInternalServerError)
		event := findSpanEvent(t, ended, "exception")
		assertNoSensitiveSpanEventText(t, event, "sql", "password", "token")
	})

	t.Run("validation error does not leak field details to span", func(t *testing.T) {
		ctx, recorder, span := newTestContextWithSpan(t)
		details := []map[string]string{{"field": "password", "label": "密码", "rule": "required", "message": "token Authorization Cookie"}}

		ValidationFailedWithErrors(ctx, "请求参数验证失败", details)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", recorder.Code)
		}
		ended := endSpan(t, span)
		if got := ended.Status().Code; got != codes.Unset {
			t.Fatalf("span status = %s, want Unset", got)
		}
		if len(ended.Events()) != 0 {
			t.Fatalf("span events = %#v, want none", ended.Events())
		}
		assertSpanIntAttribute(t, ended, spanAttrErrorCode, int(contracterrors.CodeValidationFailed))
		assertSpanIntAttribute(t, ended, spanAttrHTTPStatus, http.StatusBadRequest)
		for _, attr := range ended.Attributes() {
			text := attr.Value.Emit()
			if strings.Contains(text, "password") || strings.Contains(text, "token") || strings.Contains(text, "Authorization") || strings.Contains(text, "Cookie") {
				t.Fatalf("validation span attribute leaked field detail: %#v", ended.Attributes())
			}
		}
	})
}

func TestOKWithPaginatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	OK(ctx, map[string]any{"id": 1, "name": "Alice"})

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if recorder.Code != http.StatusOK || body["success"] != true || body["code"] != float64(contracterrors.CodeOK) || body["message"] != contractresponse.MessageOK {
		t.Fatalf("response = status %d body %#v", recorder.Code, body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["id"] != float64(1) || data["name"] != "Alice" {
		t.Fatalf("data = %#v", body["data"])
	}
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
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown tracer provider: %v", err)
		}
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
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			if got := int(attr.Value.AsInt64()); got != want {
				t.Fatalf("span attribute %s = %d, want %d", key, got, want)
			}
			return
		}
	}
	t.Fatalf("span attribute %s missing in %#v", key, span.Attributes())
}

func findSpanEvent(t *testing.T, span sdktrace.ReadOnlySpan, name string) sdktrace.Event {
	t.Helper()
	for _, event := range span.Events() {
		if event.Name == name {
			return event
		}
	}
	t.Fatalf("span event %q missing in %#v", name, span.Events())
	return sdktrace.Event{}
}

func assertSpanEventStringAttribute(t *testing.T, event sdktrace.Event, key string, want string) {
	t.Helper()
	for _, attr := range event.Attributes {
		if string(attr.Key) == key {
			if got := attr.Value.AsString(); got != want {
				t.Fatalf("span event attribute %s = %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Fatalf("span event attribute %s missing in %#v", key, event.Attributes)
}

func assertNoSensitiveSpanEventText(t *testing.T, event sdktrace.Event, forbidden ...string) {
	t.Helper()
	for _, attr := range event.Attributes {
		text := attr.Value.Emit()
		for _, item := range forbidden {
			if strings.Contains(text, item) {
				t.Fatalf("span event leaked %q in %#v", item, event.Attributes)
			}
		}
	}
}
