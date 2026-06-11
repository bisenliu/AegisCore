package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/gin-gonic/gin"
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
	return ctx, recorder
}
