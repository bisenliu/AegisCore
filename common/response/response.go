package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Success bool   `json:"success"`
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

const (
	MessageOK            = "ok"
	MessageCreated       = "created"
	MessageInternalError = "internal server error"
)

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Success: true, Code: CodeOK, Message: MessageOK, Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Code: CodeOK, Message: MessageCreated, Data: data})
}

func Fail(c *gin.Context, err error) {
	appErr := FromError(err)
	c.JSON(appErr.HTTPStatus, Envelope{Success: false, Code: appErr.Code, Message: appErr.Message})
}

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

func Forbidden(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeForbidden, formatMessage(format, args), http.StatusForbidden))
}

func Conflict(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeConflict, formatMessage(format, args), http.StatusConflict))
}

func NotFound(c *gin.Context, format string, args ...any) {
	Fail(c, NewError(CodeNotFound, formatMessage(format, args), http.StatusNotFound))
}
