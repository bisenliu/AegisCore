package pagination

// PaginatedData 将当前页数据和分页元数据包装在一起。
type PaginatedData[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}
