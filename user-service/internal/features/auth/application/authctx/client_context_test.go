package authctx

import (
	"context"
	"testing"
)

func TestClientContextFields(t *testing.T) {
	ctx := WithClientContext(context.Background(), ClientContext{ClientIP: "203.0.113.30", UserAgent: "auth-test"})

	fields := ClientContextFields(ctx)

	got := map[string]string{}
	for _, field := range fields {
		got[field.Key] = field.String
	}
	if got["client_ip"] != "203.0.113.30" || got["user_agent"] != "auth-test" {
		t.Fatalf("fields = %#v", got)
	}
}
