package errors

import stderrors "errors"

// FromError 将任意错误转换为应用错误，nil 和未知错误默认映射为内部错误。
func FromError(err error) *Error {
	if err == nil {
		// nil 错误无法生成成功的失败信封，因此按失败关闭策略映射为内部错误。
		return InternalError(nil)
	}
	var appErr *Error
	if stderrors.As(err, &appErr) {
		return appErr
	}
	return InternalError(err)
}
