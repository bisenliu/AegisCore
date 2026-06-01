package contextutil

import (
	"context"
	"testing"
)

func TestUserIDContext(t *testing.T) {
	if got, ok := UserIDFromContext(context.Background()); ok || got != "" {
		t.Fatalf("UserIDFromContext empty = %q, %v; want empty, false", got, ok)
	}

	ctx := WithUserID(context.Background(), "u-123")
	got, ok := UserIDFromContext(ctx)
	if !ok || got != "u-123" {
		t.Fatalf("UserIDFromContext = %q, %v; want u-123, true", got, ok)
	}
}
