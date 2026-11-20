package pagination

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNewPagination(t *testing.T) {
	pagination := NewPagination(20, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", true)
	require.Equal(t, 20, pagination.PageSize)
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", pagination.NextCursor)
	require.True(t, pagination.HasNext)

	empty := NewPagination(0, "", false)
	require.Equal(t, DefaultPageSize, empty.PageSize)
	require.Empty(t, empty.NextCursor)
	require.False(t, empty.HasNext)
}

func TestNewPaginatedData(t *testing.T) {
	meta := NewPagination(0, "", false)
	data := NewPaginatedData[string](nil, meta)
	require.NotNil(t, data.Items)
	require.Empty(t, data.Items)
	require.Equal(t, meta, data.Pagination)
}
