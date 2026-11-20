package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type dynamicTracerProvider struct {
	trace.TracerProvider
	provider *Provider
}

type dynamicTracer struct {
	noop.Tracer
	provider *Provider
	name     string
	options  []trace.TracerOption
}

// Tracer 返回一个动态追踪器。
//
// 内部会对配置参数进行拷贝，防止外部修改原参数切片影响已经初始化的追踪配置。
func (p dynamicTracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return dynamicTracer{provider: p.provider, name: name, options: append([]trace.TracerOption(nil), opts...)}
}

// Start 在每次 span 创建时解析当前真实 tracer。
//
// 这样 constructor 阶段保存的 tracer 不需要在 Fx OnStart 后重新获取；provider 尚未启动或已经
// Shutdown 时会自然落回 no-op tracer，避免使用已关闭 SDK provider。
func (t dynamicTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if t.provider == nil {
		return t.Tracer.Start(ctx, spanName, opts...)
	}
	return t.provider.realTracer(t.name, t.options...).Start(ctx, spanName, opts...)
}
