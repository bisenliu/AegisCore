package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *Error
		wantCode   Code
		wantStatus int
		wantMsg    string
	}{
		{name: "bad request fixed percent", err: BadRequestError("进度为 100%"), wantCode: CodeBadRequest, wantStatus: http.StatusBadRequest, wantMsg: "进度为 100%"},
		{name: "bad request formatted", err: BadRequestError("%s 必须在 %d 和 %d 之间", "人数", 2, 10), wantCode: CodeBadRequest, wantStatus: http.StatusBadRequest, wantMsg: "人数 必须在 2 和 10 之间"},
		{name: "validation failed", err: ValidationFailedError("请求参数验证失败"), wantCode: CodeValidationFailed, wantStatus: http.StatusBadRequest, wantMsg: "请求参数验证失败"},
		{name: "unauthenticated", err: UnauthenticatedError("请先登录"), wantCode: CodeUnauthenticated, wantStatus: http.StatusUnauthorized, wantMsg: "请先登录"},
		{name: "token invalid", err: TokenInvalidError(MessageAuthInvalid), wantCode: CodeTokenInvalid, wantStatus: http.StatusUnauthorized, wantMsg: MessageAuthInvalid},
		{name: "token expired", err: TokenExpiredError(MessageAuthInvalid), wantCode: CodeTokenExpired, wantStatus: http.StatusUnauthorized, wantMsg: MessageAuthInvalid},
		{name: "forbidden", err: ForbiddenError("无权访问"), wantCode: CodeForbidden, wantStatus: http.StatusForbidden, wantMsg: "无权访问"},
		{name: "conflict", err: ConflictError("当前状态不允许操作"), wantCode: CodeConflict, wantStatus: http.StatusConflict, wantMsg: "当前状态不允许操作"},
		{name: "not found", err: NotFoundError("用户不存在"), wantCode: CodeNotFound, wantStatus: http.StatusNotFound, wantMsg: "用户不存在"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode || tt.err.HTTPStatus != tt.wantStatus || tt.err.Message != tt.wantMsg {
				t.Fatalf("error = %#v", tt.err)
			}
		})
	}
}

func TestWrapInternalAndFromError(t *testing.T) {
	cause := errors.New("database down")
	err := WrapInternal(cause, "微信登录失败，请稍后再试")
	if err.Code != CodeInternalError || err.HTTPStatus != http.StatusInternalServerError || err.Message != "微信登录失败，请稍后再试" {
		t.Fatalf("WrapInternal = %#v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("WrapInternal does not unwrap cause")
	}

	appErr := NotFoundError("用户不存在")
	if FromError(appErr) != appErr {
		t.Fatal("FromError should preserve application errors")
	}

	wrapped := FromError(cause)
	if wrapped.Code != CodeInternalError || wrapped.HTTPStatus != http.StatusInternalServerError || wrapped.Message != "internal server error" {
		t.Fatalf("FromError = %#v", wrapped)
	}
	if !errors.Is(wrapped, cause) {
		t.Fatalf("FromError does not unwrap cause")
	}
}

func TestFailureResponseErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("nil failure returns internal envelope", func(t *testing.T) {
		ctx, recorder := newTestContext()

		Fail(ctx, nil)

		var envelope Envelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusInternalServerError || envelope.Success || envelope.Code != CodeInternalError || envelope.Message != MessageInternalError {
			t.Fatalf("response = status %d envelope %#v", recorder.Code, envelope)
		}
	})

	t.Run("ordinary failure omits errors", func(t *testing.T) {
		ctx, recorder := newTestContext()

		BadRequest(ctx, "请求格式错误")

		var envelope Envelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusBadRequest || envelope.Success || envelope.Code != CodeBadRequest || envelope.Message != "请求格式错误" {
			t.Fatalf("response = status %d envelope %#v", recorder.Code, envelope)
		}
		if envelope.Errors != nil {
			t.Fatalf("errors = %#v, want nil", envelope.Errors)
		}
	})

	t.Run("token invalid failure", func(t *testing.T) {
		ctx, recorder := newTestContext()

		TokenInvalid(ctx, MessageAuthInvalid)

		var envelope Envelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusUnauthorized || envelope.Success || envelope.Code != CodeTokenInvalid || envelope.Message != MessageAuthInvalid {
			t.Fatalf("response = status %d envelope %#v", recorder.Code, envelope)
		}
	})

	t.Run("token expired failure", func(t *testing.T) {
		ctx, recorder := newTestContext()

		TokenExpired(ctx, MessageAuthInvalid)

		var envelope Envelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if recorder.Code != http.StatusUnauthorized || envelope.Success || envelope.Code != CodeTokenExpired || envelope.Message != MessageAuthInvalid {
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
		if recorder.Code != http.StatusBadRequest || body["success"] != false || body["code"] != float64(CodeValidationFailed) || body["message"] != "请求参数验证失败" {
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

func TestPaginationHelpers(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		want     PaginationQuery
	}{
		{name: "missing values use defaults", page: 0, pageSize: 0, want: PaginationQuery{Page: DefaultPage, PageSize: DefaultPageSize, Offset: 0, Limit: DefaultPageSize}},
		{name: "negative values use defaults", page: -1, pageSize: -20, want: PaginationQuery{Page: DefaultPage, PageSize: DefaultPageSize, Offset: 0, Limit: DefaultPageSize}},
		{name: "explicit values calculate offset", page: 2, pageSize: 20, want: PaginationQuery{Page: 2, PageSize: 20, Offset: 20, Limit: 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePagination(tt.page, tt.pageSize)
			if got != tt.want {
				t.Fatalf("NormalizePagination = %#v, want %#v", got, tt.want)
			}
		})
	}

	pagination := NewPagination(1, 20, 128)
	if pagination.Page != 1 || pagination.PageSize != 20 || pagination.Total != 128 || pagination.TotalPages != 7 {
		t.Fatalf("pagination = %#v", pagination)
	}

	empty := NewPagination(0, 0, 0)
	if empty.Page != DefaultPage || empty.PageSize != DefaultPageSize || empty.Total != 0 || empty.TotalPages != 0 {
		t.Fatalf("empty pagination = %#v", empty)
	}

	data := NewPaginatedData[string](nil, empty)
	if data.Items == nil || len(data.Items) != 0 || data.Pagination != empty {
		t.Fatalf("paginated data = %#v", data)
	}
}

func TestOKWithPaginatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	OK(ctx, NewPaginatedData([]map[string]any{{"id": 1, "name": "Alice"}}, NewPagination(1, 20, 128)))

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if recorder.Code != http.StatusOK || body["success"] != true || body["code"] != float64(CodeOK) || body["message"] != MessageOK {
		t.Fatalf("response = status %d body %#v", recorder.Code, body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", body["data"])
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v", data["items"])
	}
	pagination, ok := data["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("pagination = %#v", data["pagination"])
	}
	if pagination["page"] != float64(1) || pagination["page_size"] != float64(20) || pagination["total"] != float64(128) || pagination["total_pages"] != float64(7) {
		t.Fatalf("pagination = %#v", pagination)
	}
}

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return ctx, recorder
}
