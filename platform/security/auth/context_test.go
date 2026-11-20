package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserIDContext(t *testing.T) {
	var nilContext context.Context
	got, ok := UserIDFromContext(nilContext)
	require.False(t, ok)
	require.Empty(t, got)
	got, ok = UserIDFromContext(context.Background())
	require.False(t, ok)
	require.Empty(t, got)
	got, ok = UserIDFromContext(WithUserID(context.Background(), ""))
	require.False(t, ok)
	require.Empty(t, got)

	ctx := WithUserID(context.Background(), "u-123")
	got, ok = UserIDFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "u-123", got)
}

func TestSessionIDContext(t *testing.T) {
	var nilContext context.Context
	got, ok := SessionIDFromContext(nilContext)
	require.False(t, ok)
	require.Empty(t, got)
	got, ok = SessionIDFromContext(context.Background())
	require.False(t, ok)
	require.Empty(t, got)
	got, ok = SessionIDFromContext(WithSessionID(context.Background(), ""))
	require.False(t, ok)
	require.Empty(t, got)

	ctx := WithSessionID(context.Background(), "s-123")
	got, ok = SessionIDFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "s-123", got)
}
