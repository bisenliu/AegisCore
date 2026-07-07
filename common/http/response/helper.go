package response

import (
	"net/http"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
)

func successEnvelope(code contracterrors.Code, message string, data any) contractresponse.Envelope {
	return contractresponse.Envelope{Success: true, Code: code, Message: message, Data: data}
}

func errorEnvelope(err *contracterrors.Error) contractresponse.Envelope {
	return contractresponse.Envelope{Success: false, Code: err.Code, Message: err.Message}
}

func validationEnvelope(message string, errors any) contractresponse.Envelope {
	return contractresponse.Envelope{Success: false, Code: contracterrors.CodeValidationFailed, Message: message, Errors: errors}
}

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
	case contracterrors.KindServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
