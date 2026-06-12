package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
)

// OK 写入携带默认成功码和消息的 200 成功响应信封。
func OK(c *gin.Context, data any) {
	JSON(c, http.StatusOK, successEnvelope(contracterrors.CodeOK, contractresponse.MessageOK, data))
}

// Created 写入 201 成功响应信封，并保持 CodeOK 作为稳定成功码。
func Created(c *gin.Context, data any) {
	JSON(c, http.StatusCreated, successEnvelope(contracterrors.CodeOK, contractresponse.MessageCreated, data))
}

// NoContent 写入 204 响应。
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// JSON 写入指定 HTTP 状态码和 JSON 载荷。
func JSON(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}
