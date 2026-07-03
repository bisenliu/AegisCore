package errors

import (
	stderrors "errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodeValues(t *testing.T) {
	tests := []struct {
		name string
		code Code
		want Code
	}{
		{name: "ok", code: CodeOK, want: 0},
		{name: "bad request", code: CodeBadRequest, want: 10000},
		{name: "validation failed", code: CodeValidationFailed, want: 10001},
		{name: "unauthenticated", code: CodeUnauthenticated, want: 20000},
		{name: "token invalid", code: CodeTokenInvalid, want: 20001},
		{name: "token expired", code: CodeTokenExpired, want: 20002},
		{name: "token revoked", code: CodeTokenRevoked, want: 20003},
		{name: "mfa required", code: CodeMFARequired, want: 20004},
		{name: "user account locked", code: CodeUserAccountLocked, want: 20005},
		{name: "password change required", code: CodePasswordChangeRequired, want: 20006},
		{name: "forbidden", code: CodeForbidden, want: 30000},
		{name: "conflict", code: CodeConflict, want: 40000},
		{name: "not found", code: CodeNotFound, want: 50000},
		{name: "internal error", code: CodeInternalError, want: 90000},
		{name: "service unavailable", code: CodeServiceUnavailable, want: 90001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.code)
		})
	}
}

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
		{name: "token invalid", err: TokenInvalidError("登录状态无效或已过期，请重新登录"), wantCode: CodeTokenInvalid, wantStatus: http.StatusUnauthorized, wantMsg: "登录状态无效或已过期，请重新登录"},
		{name: "token expired", err: TokenExpiredError("登录状态无效或已过期，请重新登录"), wantCode: CodeTokenExpired, wantStatus: http.StatusUnauthorized, wantMsg: "登录状态无效或已过期，请重新登录"},
		{name: "forbidden", err: ForbiddenError("无权访问"), wantCode: CodeForbidden, wantStatus: http.StatusForbidden, wantMsg: "无权访问"},
		{name: "conflict", err: ConflictError("当前状态不允许操作"), wantCode: CodeConflict, wantStatus: http.StatusConflict, wantMsg: "当前状态不允许操作"},
		{name: "not found", err: NotFoundError("用户不存在"), wantCode: CodeNotFound, wantStatus: http.StatusNotFound, wantMsg: "用户不存在"},
		{name: "service unavailable", err: ServiceUnavailableError("服务繁忙，请稍后重试"), wantCode: CodeServiceUnavailable, wantStatus: http.StatusServiceUnavailable, wantMsg: "服务繁忙，请稍后重试"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantCode, tt.err.Code)
			require.Equal(t, tt.wantStatus, tt.err.HTTPStatus)
			require.Equal(t, tt.wantMsg, tt.err.Message)
		})
	}
}

func TestWrapInternalAndFromError(t *testing.T) {
	cause := stderrors.New("database down")
	err := WrapInternal(cause, "微信登录失败，请稍后再试")
	require.Equal(t, CodeInternalError, err.Code)
	require.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	require.Equal(t, "微信登录失败，请稍后再试", err.Message)
	require.ErrorIs(t, err, cause)

	appErr := NotFoundError("用户不存在")
	require.Same(t, appErr, FromError(appErr))

	wrapped := FromError(cause)
	require.Equal(t, CodeInternalError, wrapped.Code)
	require.Equal(t, http.StatusInternalServerError, wrapped.HTTPStatus)
	require.Equal(t, MessageInternalError, wrapped.Message)
	require.ErrorIs(t, wrapped, cause)
}

func TestWrapServiceUnavailable(t *testing.T) {
	cause := stderrors.New("argon2 queue full")
	err := WrapServiceUnavailable(cause, "认证服务繁忙，请稍后重试")
	require.Equal(t, CodeServiceUnavailable, err.Code)
	require.Equal(t, http.StatusServiceUnavailable, err.HTTPStatus)
	require.Equal(t, "认证服务繁忙，请稍后重试", err.Message)
	require.ErrorIs(t, err, cause)
}

func TestFromErrorNil(t *testing.T) {
	err := FromError(nil)
	require.Equal(t, CodeInternalError, err.Code)
	require.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	require.Equal(t, MessageInternalError, err.Message)
}

func TestNilErrorReceiver(t *testing.T) {
	var err *Error
	require.Empty(t, err.Error())
	require.Nil(t, err.Unwrap())
}
