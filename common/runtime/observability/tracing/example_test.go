package tracing_test

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/observability/tracing"
)

// ExampleProvider_disabled 演示 disabled provider 的推荐用法。
//
// 即使 tracing 被禁用，NewProvider 仍会返回一个非 nil provider，调用方可以继续把它注入
// instrumentation 并按统一路径创建 span。disabled provider 不需要 OTLP endpoint，也不会连接
// 真实 collector；这里仍显式调用 Shutdown，展示调用方拥有普通 constructor 返回资源的关闭职责。
func ExampleProvider_disabled() {
	// disabled 配置不需要 OTLPEndpoint，适合本地示例、测试和禁用 tracing 的运行环境。
	provider, err := tracing.NewProvider(context.Background(), tracing.Options{
		Config: config.TracingConfig{
			Enabled:     false,
			SampleRatio: 1.0,
		},
		ServiceName: "aegiscore-example",
		Environment: "local",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	// 普通 constructor 立即完成 provider 构造，调用方负责在结束时显式关闭。
	defer func() { _ = provider.Shutdown(context.Background()) }()

	// disabled provider 仍允许创建 span，但采样状态固定为 false，不会导出到 OTLP。
	_, span := provider.Tracer("example").Start(context.Background(), "disabled-operation")
	defer span.End()

	fmt.Println(span.SpanContext().IsSampled())
	// Output: false
}

// ExampleProvider_Tracer 演示通过 Provider.Tracer 创建 span。
//
// 示例使用 disabled provider 避免网络依赖，但仍能证明 SDK provider 已经启动并返回有效
// SpanContext。业务代码无需根据 tracing 是否启用切换不同 instrumentation 路径。
func ExampleProvider_Tracer() {
	// SampleRatio 在 disabled 模式下不会导致真实导出；这里保留 1.0 只展示配置形状。
	provider, err := tracing.NewProvider(context.Background(), tracing.Options{
		Config: config.TracingConfig{
			Enabled:     false,
			SampleRatio: 1.0,
		},
		ServiceName: "aegiscore-example",
		Environment: "local",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Tracer 返回动态 tracer；普通 constructor 已启动 provider，因此这里会创建有效 span ID。
	_, span := provider.Tracer("example").Start(context.Background(), "operation")
	defer span.End()

	fmt.Println(span.SpanContext().IsValid())
	// Output: true
}

// ExampleProvider_TextMapPropagator 演示 W3C trace context 的注入和提取。
//
// propagator 与 exporter 生命周期解耦：即使 provider 不连接真实 OTLP endpoint，也可以稳定执行
// TraceContext 和 Baggage 的 inject/extract，便于 HTTP、Redis、Ent 等 instrumentation 在统一 facade
// 上传播上下文。
func ExampleProvider_TextMapPropagator() {
	provider, err := tracing.NewProvider(context.Background(), tracing.Options{
		Config: config.TracingConfig{
			Enabled:     false,
			SampleRatio: 1.0,
		},
		ServiceName: "aegiscore-example",
		Environment: "local",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	// 构造一个稳定的 remote SpanContext，避免示例输出随机 trace/span ID。
	traceID, _ := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	spanID, _ := trace.SpanIDFromHex("0011223344556677")
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)
	carrier := propagation.MapCarrier{}

	// Inject 写入标准 traceparent header，Extract 再从 carrier 还原 trace context。
	provider.TextMapPropagator().Inject(ctx, carrier)
	extracted := provider.TextMapPropagator().Extract(context.Background(), carrier)

	fmt.Println(strings.HasPrefix(carrier.Get("traceparent"), "00-00112233445566778899aabbccddeeff-0011223344556677-"))
	fmt.Println(trace.SpanContextFromContext(extracted).TraceID().String())
	// Output:
	// true
	// 00112233445566778899aabbccddeeff
}

// ExampleProvider_Shutdown 演示 Shutdown 的幂等语义。
//
// Shutdown 负责关闭底层 SDK provider、batch processor 和 exporter，并把动态 tracer 恢复为 no-op。
// 对已经关闭的 provider 再次调用 Shutdown 必须安全返回 nil，方便 lifecycle rollback 和显式清理
// 共用同一个关闭入口。
func ExampleProvider_Shutdown() {
	provider, err := tracing.NewProvider(context.Background(), tracing.Options{
		Config: config.TracingConfig{
			Enabled:     false,
			SampleRatio: 1.0,
		},
		ServiceName: "aegiscore-example",
		Environment: "local",
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	// 第一次调用关闭真实 SDK provider；第二次调用验证重复关闭不会报错。
	fmt.Println(provider.Shutdown(context.Background()) == nil)
	fmt.Println(provider.Shutdown(context.Background()) == nil)
	// Output:
	// true
	// true
}
