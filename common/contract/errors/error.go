package errors

import stderrors "errors"

// Error 表示系统统一处理的业务与应用错误。
// 该结构体包含对前端展示的错误码、提示语，以及仅在服务端日志中记录的内部底层原因（Cause，不会直接暴露给前端用户）。
type Error struct {
	Kind    Kind
	Reason  Reason
	Code    Code
	Message string
	Cause   error
}

// Error 返回对外展示的用户提示信息。若指针为空则返回空字符串，避免在打印日志时发生空指针异常。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Unwrap 返回产生该错误的底层内部原因，支持标准库 errors.Is 和 errors.As 进行链式排查。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is 比较当前错误是否与目标错误相符。
// 支持按错误大类（Kind）或具体业务原因（Reason）进行归类比对，只要任意一项匹配且未产生冲突即视为同类错误。
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	var targetErr *Error
	if !stderrors.As(target, &targetErr) || targetErr == nil {
		return false
	}
	matched := false
	if targetErr.Kind != "" {
		if e.Kind != targetErr.Kind {
			return false
		}
		matched = true
	}
	if targetErr.Reason != "" {
		if e.Reason != targetErr.Reason {
			return false
		}
		matched = true
	}
	return matched
}
