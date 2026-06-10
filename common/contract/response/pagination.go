package response

const (
	// DefaultPageSize 是分页大小缺失或无效时使用的兜底每页数量。
	DefaultPageSize = 10
	// MaxPageSize 是分页大小允许的最大值。
	MaxPageSize = 100
)

// Pagination 描述分页集合响应的元数据。
type Pagination struct {
	PageSize   int    `json:"page_size" example:"50"`
	NextCursor string `json:"next_cursor,omitempty" example:"0190c8d2-8d8a-7a01-9f43-0f91fb4e2b7c"`
	HasNext    bool   `json:"has_next" example:"true"`
}

// PaginatedData 将当前页数据和分页元数据包装在一起。
type PaginatedData[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// NormalizePageSize 将无效或过大的分页大小修正为公开契约允许的范围。
func NormalizePageSize(pageSize int) int {
	if pageSize < 1 {
		return DefaultPageSize
	}
	if pageSize > MaxPageSize {
		return MaxPageSize
	}
	return pageSize
}

// NewPagination 使用 keyset 分页输入创建响应分页元数据。
func NewPagination(pageSize int, nextCursor string, hasNext bool) Pagination {
	return Pagination{PageSize: NormalizePageSize(pageSize), NextCursor: nextCursor, HasNext: hasNext}
}

// NewPaginatedData 创建分页载荷，并保证空结果集仍输出 JSON 数组。
func NewPaginatedData[T any](items []T, pagination Pagination) PaginatedData[T] {
	if items == nil {
		// JSON 客户端通常期望集合字段保持数组，而不是在空结果时变为 null。
		items = []T{}
	}
	return PaginatedData[T]{Items: items, Pagination: pagination}
}
