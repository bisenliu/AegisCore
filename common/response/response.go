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

type Pagination struct {
	Page       int `json:"page" example:"1"`
	PageSize   int `json:"page_size" example:"20"`
	Total      int `json:"total" example:"128"`
	TotalPages int `json:"total_pages" example:"7"`
}

type PaginatedData[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type PaginationQuery struct {
	Page     int
	PageSize int
	Offset   int
	Limit    int
}

const (
	MessageOK            = "ok"
	MessageCreated       = "created"
	MessageInternalError = "internal server error"
	DefaultPage          = 1
	DefaultPageSize      = 10
)

func NormalizePagination(page, pageSize int) PaginationQuery {
	if page < 1 {
		page = DefaultPage
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	return PaginationQuery{Page: page, PageSize: pageSize, Offset: (page - 1) * pageSize, Limit: pageSize}
}

func NewPagination(page, pageSize, total int) Pagination {
	query := NormalizePagination(page, pageSize)
	page = query.Page
	pageSize = query.PageSize
	if total < 1 {
		return Pagination{Page: page, PageSize: pageSize, Total: 0, TotalPages: 0}
	}
	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}
	return Pagination{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
}

func NewPaginatedData[T any](items []T, pagination Pagination) PaginatedData[T] {
	if items == nil {
		items = []T{}
	}
	return PaginatedData[T]{Items: items, Pagination: pagination}
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
