package pagination

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
