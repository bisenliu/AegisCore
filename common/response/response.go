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
