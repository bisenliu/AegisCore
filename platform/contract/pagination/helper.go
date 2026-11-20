package pagination

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
