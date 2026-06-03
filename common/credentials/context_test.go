package credentials

import (
	"context"
	"testing"
)

func TestUserIDContext(t *testing.T) {
	if got, ok := UserIDFromContext(nil); ok || got != "" {
		t.Fatalf("UserIDFromContext nil = %q, %v; want empty, false", got, ok)
	}
	if got, ok := UserIDFromContext(context.Background()); ok || got != "" {
		t.Fatalf("UserIDFromContext empty = %q, %v; want empty, false", got, ok)
	}
	if got, ok := UserIDFromContext(WithUserID(context.Background(), "")); ok || got != "" {
		t.Fatalf("UserIDFromContext blank = %q, %v; want empty, false", got, ok)
	}

	ctx := WithUserID(context.Background(), "u-123")
	got, ok := UserIDFromContext(ctx)
	if !ok || got != "u-123" {
		t.Fatalf("UserIDFromContext = %q, %v; want u-123, true", got, ok)
	}
}

func TestSessionIDContext(t *testing.T) {
	if got, ok := SessionIDFromContext(nil); ok || got != "" {
		t.Fatalf("SessionIDFromContext nil = %q, %v; want empty, false", got, ok)
	}
	if got, ok := SessionIDFromContext(context.Background()); ok || got != "" {
		t.Fatalf("SessionIDFromContext empty = %q, %v; want empty, false", got, ok)
	}
	if got, ok := SessionIDFromContext(WithSessionID(context.Background(), "")); ok || got != "" {
		t.Fatalf("SessionIDFromContext blank = %q, %v; want empty, false", got, ok)
	}

	ctx := WithSessionID(context.Background(), "s-123")
	got, ok := SessionIDFromContext(ctx)
	if !ok || got != "s-123" {
		t.Fatalf("SessionIDFromContext = %q, %v; want s-123, true", got, ok)
	}
}
