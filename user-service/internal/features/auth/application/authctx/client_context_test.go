package authctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientContextFields(t *testing.T) {
	ctx := WithClientContext(context.Background(), ClientContext{ClientIP: "203.0.113.30", UserAgent: "auth-test"})

	fields := ClientContextFields(ctx)

	got := map[string]string{}
	for _, field := range fields {
		got[field.Key] = field.String
	}
	require.False(t, got["client_ip"] != "203.0.113.30" || got["user_agent"] != "auth-test",
		"fields = %#v", got)

}
