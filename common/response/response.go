package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Success bool        `json:"success"`
	Code    Code        `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Code: CodeOK, Message: "ok", Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Code: CodeOK, Message: "created", Data: data})
}

func Fail(c *gin.Context, err error) {
	appErr := FromError(err)
	c.JSON(appErr.HTTPStatus, Envelope{Success: false, Code: appErr.Code, Message: appErr.Message})
}

func BadRequest(c *gin.Context, message string) {
	Fail(c, BadRequestError(message))
}

func NotFound(c *gin.Context, message string) {
	Fail(c, NotFoundError(message))
}
