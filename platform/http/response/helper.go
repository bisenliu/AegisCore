package response

import (
	"net/http"

	contracterrors "github.com/aegiscore/platform/contract/errors"
	contractresponse "github.com/aegiscore/platform/contract/response"
)

// successEnvelope 打包业务处理成功的返回结果，统一包含成功标记、状态码、提示语及具体数据内容。
func successEnvelope(code contracterrors.Code, message string, data any) contractresponse.Envelope {
	return contractresponse.Envelope{Success: true, Code: code, Message: message, Data: data}
}

// errorEnvelope 打包业务处理失败的返回结果，仅包含安全的对外状态码和错误原因提示。
func errorEnvelope(err *contracterrors.Error) contractresponse.Envelope {
	return contractresponse.Envelope{Success: false, Code: err.Code, Message: err.Message}
}

// validationEnvelope 打包输入内容检查不合格时的返回结果，清晰列出具体哪个输入项填写有误以及不合格的原因。
func validationEnvelope(message string, errors any) contractresponse.Envelope {
	return contractresponse.Envelope{Success: false, Code: contracterrors.CodeValidationFailed, Message: message, Errors: errors}
}

// statusCode 根据业务错误的类型（如未登录、无权限、找不到资源或系统故障），匹配对应的网络状态标识（如 401、403、404 或 500）。
func statusCode(err *contracterrors.Error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	switch err.Kind {
	case contracterrors.KindBadRequest, contracterrors.KindValidation:
		return http.StatusBadRequest
	case contracterrors.KindUnauthenticated:
		return http.StatusUnauthorized
	case contracterrors.KindForbidden:
		return http.StatusForbidden
	case contracterrors.KindNotFound:
		return http.StatusNotFound
	case contracterrors.KindConflict:
		return http.StatusConflict
	case contracterrors.KindRateLimited:
		return http.StatusTooManyRequests
	case contracterrors.KindPayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case contracterrors.KindServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
