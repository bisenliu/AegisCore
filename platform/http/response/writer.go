package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/platform/contract/errors"
	contractresponse "github.com/aegiscore/platform/contract/response"
)

// OK 向客户端返回 HTTP 200 成功响应，并携带标准的数据结构（含业务状态码 0、默认提示语及数据体）。
func OK(c *gin.Context, data any) {
	JSON(c, http.StatusOK, successEnvelope(contracterrors.CodeOK, contractresponse.MessageOK, data))
}

// Created 向客户端返回 HTTP 201 创建成功响应，表明新增资源已成功入库。
func Created(c *gin.Context, data any) {
	JSON(c, http.StatusCreated, successEnvelope(contracterrors.CodeOK, contractresponse.MessageCreated, data))
}

// NoContent 向客户端返回 HTTP 204 响应（无内容），适用于删除成功等无需返回数据体的操作。
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// JSON 输出指定 HTTP 状态码与 JSON 格式的返回内容。
func JSON(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}
