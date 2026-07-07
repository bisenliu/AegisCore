package errors

import stderrors "errors"

// Error 描述可渲染为响应信封的应用错误。
// Cause 保留内部错误供 errors.Is/As 使用，但不会暴露给客户端。
type Error struct {
	Kind    Kind
	Reason  Reason
	Code    Code
	Message string
	Cause   error
}

// Error 返回对外展示的错误消息，并允许 nil receiver 以兼容防御性日志路径。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Unwrap 返回内部原因错误，使调用方可以使用 errors.Is 和 errors.As。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is 支持按错误类别或原因进行稳定语义匹配。
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
