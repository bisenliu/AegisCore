package command

import (
	"context"

	"go.uber.org/zap"
)

// ClientContext 表示认证安全审计需要的客户端上下文。
type ClientContext struct {
	ClientIP  string
	UserAgent string
}

type clientContextKey struct{}

// WithClientContext 返回携带客户端审计上下文的 context。
func WithClientContext(ctx context.Context, meta ClientContext) context.Context {
	return context.WithValue(ctx, clientContextKey{}, meta)
}

// ClientContextFromContext 从 ctx 读取客户端审计上下文。
func ClientContextFromContext(ctx context.Context) (ClientContext, bool) {
	if ctx == nil {
		return ClientContext{}, false
	}
	meta, ok := ctx.Value(clientContextKey{}).(ClientContext)
	return meta, ok
}

func clientContextFields(ctx context.Context) []zap.Field {
	meta, ok := ClientContextFromContext(ctx)
	if !ok {
		return nil
	}
	fields := make([]zap.Field, 0, 2)
	if meta.ClientIP != "" {
		fields = append(fields, zap.String("client_ip", meta.ClientIP))
	}
	if meta.UserAgent != "" {
		fields = append(fields, zap.String("user_agent", meta.UserAgent))
	}
	return fields
}
