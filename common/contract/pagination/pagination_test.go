package pagination

import "testing"

func TestNormalizePageSize(t *testing.T) {
	tests := []struct {
		name     string
		pageSize int
		want     int
	}{
		{name: "missing values use defaults", pageSize: 0, want: DefaultPageSize},
		{name: "negative values use defaults", pageSize: -20, want: DefaultPageSize},
		{name: "explicit value is preserved", pageSize: 20, want: 20},
		{name: "oversized value is capped", pageSize: 101, want: MaxPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePageSize(tt.pageSize)
			if got != tt.want {
				t.Fatalf("NormalizePageSize = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewPagination(t *testing.T) {
	pagination := NewPagination(20, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", true)
	if pagination.PageSize != 20 || pagination.NextCursor != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || !pagination.HasNext {
		t.Fatalf("pagination = %#v", pagination)
	}

	empty := NewPagination(0, "", false)
	if empty.PageSize != DefaultPageSize || empty.NextCursor != "" || empty.HasNext {
		t.Fatalf("empty pagination = %#v", empty)
	}
}

func TestNewPaginatedData(t *testing.T) {
	meta := NewPagination(0, "", false)
	data := NewPaginatedData[string](nil, meta)
	if data.Items == nil || len(data.Items) != 0 || data.Pagination != meta {
		t.Fatalf("paginated data = %#v", data)
	}
}
