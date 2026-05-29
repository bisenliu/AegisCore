package response

import (
	"errors"
	"net/http"
	"testing"
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
