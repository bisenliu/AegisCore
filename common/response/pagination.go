package response

const (
	DefaultPage     = 1
	DefaultPageSize = 10
)

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
