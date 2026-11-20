package errors

import (
	stderrors "errors"
	"reflect"
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
		{name: "rate limited", code: CodeRateLimited, want: 60000},
		{name: "request body too large", code: CodeRequestBodyTooLarge, want: 60001},
		{name: "internal error", code: CodeInternalError, want: 90000},
		{name: "service unavailable", code: CodeServiceUnavailable, want: 90001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.code)
		})
	}
}

func TestErrorShapeDoesNotExposeTransportStatus(t *testing.T) {
	errType := reflect.TypeOf(Error{})
	_, ok := errType.FieldByName("HTTP" + "Status")
	require.False(t, ok)
	requireErrorField(t, errType, "Kind")
	requireErrorField(t, errType, "Reason")
	requireErrorField(t, errType, "Code")
	requireErrorField(t, errType, "Message")
	requireErrorField(t, errType, "Cause")
}

func requireErrorField(t *testing.T, errType reflect.Type, name string) {
	t.Helper()
	_, ok := errType.FieldByName(name)
	require.True(t, ok, "missing Error.%s", name)
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *Error
		wantKind   Kind
		wantReason Reason
		wantCode   Code
		wantMsg    string
	}{
		{name: "bad request fixed percent", err: BadRequestError("进度为 100%"), wantKind: KindBadRequest, wantReason: ReasonBadRequest, wantCode: CodeBadRequest, wantMsg: "进度为 100%"},
		{name: "bad request formatted", err: BadRequestError("%s 必须在 %d 和 %d 之间", "人数", 2, 10), wantKind: KindBadRequest, wantReason: ReasonBadRequest, wantCode: CodeBadRequest, wantMsg: "人数 必须在 2 和 10 之间"},
		{name: "validation failed", err: ValidationFailedError("请求参数验证失败"), wantKind: KindValidation, wantReason: ReasonValidationFailed, wantCode: CodeValidationFailed, wantMsg: "请求参数验证失败"},
		{name: "unauthenticated", err: UnauthenticatedError("请先登录"), wantKind: KindUnauthenticated, wantReason: ReasonUnauthenticated, wantCode: CodeUnauthenticated, wantMsg: "请先登录"},
		{name: "token invalid", err: TokenInvalidError("登录状态无效或已过期，请重新登录"), wantKind: KindUnauthenticated, wantReason: ReasonTokenInvalid, wantCode: CodeTokenInvalid, wantMsg: "登录状态无效或已过期，请重新登录"},
		{name: "token expired", err: TokenExpiredError("登录状态无效或已过期，请重新登录"), wantKind: KindUnauthenticated, wantReason: ReasonTokenExpired, wantCode: CodeTokenExpired, wantMsg: "登录状态无效或已过期，请重新登录"},
		{name: "forbidden", err: ForbiddenError("无权访问"), wantKind: KindForbidden, wantReason: ReasonForbidden, wantCode: CodeForbidden, wantMsg: "无权访问"},
		{name: "conflict", err: ConflictError("当前状态不允许操作"), wantKind: KindConflict, wantReason: ReasonConflict, wantCode: CodeConflict, wantMsg: "当前状态不允许操作"},
		{name: "not found", err: NotFoundError("用户不存在"), wantKind: KindNotFound, wantReason: ReasonNotFound, wantCode: CodeNotFound, wantMsg: "用户不存在"},
		{name: "rate limited", err: RateLimitedError("请求过于频繁"), wantKind: KindRateLimited, wantReason: ReasonRateLimited, wantCode: CodeRateLimited, wantMsg: "请求过于频繁"},
		{name: "request body too large", err: RequestBodyTooLargeError(), wantKind: KindPayloadTooLarge, wantReason: ReasonRequestBodyTooLarge, wantCode: CodeRequestBodyTooLarge, wantMsg: MessageRequestBodyTooLarge},
		{name: "service unavailable", err: ServiceUnavailableError("服务繁忙，请稍后重试"), wantKind: KindServiceUnavailable, wantReason: ReasonServiceUnavailable, wantCode: CodeServiceUnavailable, wantMsg: "服务繁忙，请稍后重试"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantKind, tt.err.Kind)
			require.Equal(t, tt.wantReason, tt.err.Reason)
			require.Equal(t, tt.wantCode, tt.err.Code)
			require.Equal(t, tt.wantMsg, tt.err.Message)
		})
	}
}

func TestWrapRequestBodyTooLarge(t *testing.T) {
	cause := stderrors.New("http: request body too large")
	err := WrapRequestBodyTooLarge(cause)
	require.Equal(t, KindPayloadTooLarge, err.Kind)
	require.Equal(t, ReasonRequestBodyTooLarge, err.Reason)
	require.Equal(t, CodeRequestBodyTooLarge, err.Code)
	require.Equal(t, MessageRequestBodyTooLarge, err.Message)
	require.ErrorIs(t, err, cause)
}

func TestWrapInternalAndFromError(t *testing.T) {
	cause := stderrors.New("database down")
	err := WrapInternal(cause, "微信登录失败，请稍后再试")
	require.Equal(t, KindInternal, err.Kind)
	require.Equal(t, ReasonInternalError, err.Reason)
	require.Equal(t, CodeInternalError, err.Code)
	require.Equal(t, "微信登录失败，请稍后再试", err.Message)
	require.ErrorIs(t, err, cause)

	appErr := NotFoundError("用户不存在")
	require.Same(t, appErr, FromError(appErr))

	wrapped := FromError(cause)
	require.Equal(t, KindInternal, wrapped.Kind)
	require.Equal(t, ReasonInternalError, wrapped.Reason)
	require.Equal(t, CodeInternalError, wrapped.Code)
	require.Equal(t, MessageInternalError, wrapped.Message)
	require.ErrorIs(t, wrapped, cause)
}

func TestWrapServiceUnavailable(t *testing.T) {
	cause := stderrors.New("dependency busy")
	err := WrapServiceUnavailable(cause, "认证服务繁忙，请稍后重试")
	require.Equal(t, KindServiceUnavailable, err.Kind)
	require.Equal(t, ReasonServiceUnavailable, err.Reason)
	require.Equal(t, CodeServiceUnavailable, err.Code)
	require.Equal(t, "认证服务繁忙，请稍后重试", err.Message)
	require.ErrorIs(t, err, cause)
}

func TestFromErrorNil(t *testing.T) {
	err := FromError(nil)
	require.Equal(t, KindInternal, err.Kind)
	require.Equal(t, ReasonInternalError, err.Reason)
	require.Equal(t, CodeInternalError, err.Code)
	require.Equal(t, MessageInternalError, err.Message)
}

func TestFromErrorPreservesWrappedApplicationError(t *testing.T) {
	appErr := TokenInvalidError("登录状态无效")
	err := stderrors.Join(stderrors.New("context"), appErr)

	got := FromError(err)

	require.Same(t, appErr, got)
	require.Equal(t, KindUnauthenticated, got.Kind)
	require.Equal(t, ReasonTokenInvalid, got.Reason)
	require.Equal(t, CodeTokenInvalid, got.Code)
	require.Equal(t, "登录状态无效", got.Message)
}

func TestErrorIsMatchesKindAndReason(t *testing.T) {
	err := TokenInvalidError("登录状态无效")

	require.ErrorIs(t, err, &Error{Kind: KindUnauthenticated})
	require.ErrorIs(t, err, &Error{Reason: ReasonTokenInvalid})
	require.ErrorIs(t, err, &Error{Kind: KindUnauthenticated, Reason: ReasonTokenInvalid})
	require.NotErrorIs(t, err, &Error{Kind: KindForbidden})
	require.NotErrorIs(t, err, &Error{Kind: KindUnauthenticated, Reason: ReasonTokenExpired})
}

func TestErrorAsFindsWrappedApplicationError(t *testing.T) {
	cause := stderrors.New("context")
	appErr := Wrap(cause, KindConflict, ReasonConflict, CodeConflict, "资源冲突")
	wrapped := stderrors.Join(stderrors.New("outer"), appErr)

	var got *Error
	require.ErrorAs(t, wrapped, &got)
	require.Same(t, appErr, got)
	require.ErrorIs(t, got, cause)
}

func TestNilErrorReceiver(t *testing.T) {
	var err *Error
	require.Empty(t, err.Error())
	require.Nil(t, err.Unwrap())
}
