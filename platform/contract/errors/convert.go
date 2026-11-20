package errors

import stderrors "errors"

// FromError 将任意错误转换为统一的业务错误格式；传入空错误或未识别的底层错误时，默认作为系统内部错误处理以保障安全。
func FromError(err error) *Error {
	if err == nil {
		// 传入空错误却要生成错误响应自相矛盾，为防止误将失败请求当成成功返回，默认按系统内部错误处理。
		return InternalError(nil)
	}
	var appErr *Error
	if stderrors.As(err, &appErr) {
		return appErr
	}
	return InternalError(err)
}
