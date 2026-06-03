package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func BadRequest(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeBadRequest, formatMessage(format, args), http.StatusBadRequest))
}

func ValidationFailed(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeValidationFailed, formatMessage(format, args), http.StatusBadRequest))
}

func ValidationFailedWithErrors(c *gin.Context, message string, errors any) {
	c.JSON(http.StatusBadRequest, Envelope{Success: false, Code: CodeValidationFailed, Message: message, Errors: errors})
}

func Unauthenticated(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeUnauthenticated, formatMessage(format, args), http.StatusUnauthorized))
}

func TokenInvalid(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeTokenInvalid, formatMessage(format, args), http.StatusUnauthorized))
}

func TokenExpired(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeTokenExpired, formatMessage(format, args), http.StatusUnauthorized))
}

func Forbidden(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeForbidden, formatMessage(format, args), http.StatusForbidden))
}

func Conflict(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeConflict, formatMessage(format, args), http.StatusConflict))
}

func NotFound(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeNotFound, formatMessage(format, args), http.StatusNotFound))
}
