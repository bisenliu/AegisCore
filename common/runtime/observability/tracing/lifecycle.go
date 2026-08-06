package tracing

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/aegiscore/common/runtime/config"
)

// start 使用 lifecycle context 初始化当前 Provider 的底层 SDK provider。
//
// 该函数刻意保持包内可见，避免包外调用方绕过 NewProvider 或 NewTracingProvider 形成双 lifecycle
// 所有权。重复启动会返回 errProviderAlreadyStarted，并保持已有 SDK provider 不变。
func (p *Provider) start(ctx context.Context, cfg config.TracingConfig, sampler sdktrace.Sampler, createExporter exporterFactory) error {
	if p == nil {
		return errors.New("tracing provider is required")
	}
	if ctx == nil {
		return errors.New("tracing provider context is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tracerProvider != nil {
		return errProviderAlreadyStarted
	}
	if p.resource == nil {
		return errors.New("tracing resource is required")
	}
	tp, err := newTracerProvider(ctx, cfg, p.resource, sampler, createExporter)
	if err != nil {
		return err
	}
	p.tracerProvider = tp
	return nil
}

// newTracerProvider 根据启用状态创建 SDK TracerProvider。
//
// disabled 模式只创建带 resource 和 sampler 的本地 provider，不创建 exporter；enabled 模式必须通过
// createExporter 构造 batch processor 依赖的 exporter。
func newTracerProvider(
	ctx context.Context,
	cfg config.TracingConfig,
	res *resource.Resource,
	sampler sdktrace.Sampler,
	createExporter exporterFactory,
) (*sdktrace.TracerProvider, error) {
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}
	if !cfg.Enabled {
		return sdktrace.NewTracerProvider(options...), nil
	}
	if createExporter == nil {
		return nil, errors.New("tracing exporter factory is required")
	}
	traceExporter, err := createExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	options = append(options, sdktrace.WithBatcher(traceExporter))
	return sdktrace.NewTracerProvider(options...), nil
}
