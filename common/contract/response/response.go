package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope 是成功和失败 API 响应共用的 HTTP 响应信封。
type Envelope struct {
	Success bool   `json:"success"`
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// OK 写入携带默认成功码和消息的 200 成功响应信封。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Success: true, Code: CodeOK, Message: MessageOK, Data: data})
}

// Created 写入 201 成功响应信封，并保持 CodeOK 作为稳定成功码。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Code: CodeOK, Message: MessageCreated, Data: data})
}

// Fail 将 err 转换为应用错误，并写入对应失败响应信封。
func Fail(c *gin.Context, err error) {
	appErr := FromError(err)
	// 字段级校验明细由 ValidationFailedWithErrors 输出，通用失败保持信封简洁。
	c.JSON(appErr.HTTPStatus, Envelope{Success: false, Code: appErr.Code, Message: appErr.Message})
}
