package response

const (
	// DefaultPage 是分页参数缺失或无效时使用的首页页码。
	DefaultPage = 1
	// DefaultPageSize 是分页大小缺失或无效时使用的兜底每页数量。
	DefaultPageSize = 10
)

// Pagination 描述分页集合响应的元数据。
type Pagination struct {
	Page       int `json:"page" example:"1"`
	PageSize   int `json:"page_size" example:"20"`
	Total      int `json:"total" example:"128"`
	TotalPages int `json:"total_pages" example:"7"`
}

// PaginatedData 将当前页数据和分页元数据包装在一起。
type PaginatedData[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// PaginationQuery 是规范化后的请求分页参数，包含派生的 offset 和 limit。
type PaginationQuery struct {
	Page     int
	PageSize int
	Offset   int
	Limit    int
}

// NormalizePagination 将无效分页输入修正为仓储层安全默认值，并计算 offset/limit。
func NormalizePagination(page, pageSize int) PaginationQuery {
	if page < 1 {
		// 无效页码按未传入处理，使调用方稳定获得第一页响应。
		page = DefaultPage
	}
	if pageSize < 1 {
		// 非正数分页大小会导致 offset/limit 不可用，因此使用公开默认值。
		pageSize = DefaultPageSize
	}
	return PaginationQuery{Page: page, PageSize: pageSize, Offset: (page - 1) * pageSize, Limit: pageSize}
}

// NewPagination 使用规范化分页输入创建响应分页元数据。
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

// NewPaginatedData 创建分页载荷，并保证空结果集仍输出 JSON 数组。
func NewPaginatedData[T any](items []T, pagination Pagination) PaginatedData[T] {
	if items == nil {
		// JSON 客户端通常期望集合字段保持数组，而不是在空结果时变为 null。
		items = []T{}
	}
	return PaginatedData[T]{Items: items, Pagination: pagination}
}
