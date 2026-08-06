package tracing

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/aegiscore/common/runtime/config"
)

const (
	errProviderAlreadyStarted = lifecycleError("tracing provider is already started")
)

type lifecycleError string

// Error 返回 lifecycle sentinel 的稳定错误文本。
func (e lifecycleError) Error() string { return string(e) }

// Options 描述构造 OpenTelemetry tracer provider 所需的跨服务运行时输入。
type Options struct {
	Config      config.TracingConfig
	ServiceName string
	Environment string
	Version     string
	InstanceID  string
}

// Provider 持有 tracing runtime 的 SDK provider、resource 和传播器。
type Provider struct {
	mu             sync.RWMutex
	tracerProvider *sdktrace.TracerProvider
	resource       *resource.Resource
	propagator     propagation.TextMapPropagator
}

// NewProvider 基于配置创建本进程 OpenTelemetry tracer provider。
func NewProvider(ctx context.Context, opts Options) (*Provider, error) {
	return newProvider(ctx, opts, newOTLPExporter)
}

type exporterFactory func(context.Context, config.TracingConfig) (sdktrace.SpanExporter, error)

// newProvider 创建并立即启动普通 Go provider。
//
// 与 Fx adapter 不同，该路径在 constructor 返回前完成 exporter 和 SDK provider 初始化；调用方获得
// provider 后拥有显式 Shutdown 责任。
func newProvider(ctx context.Context, opts Options, createExporter exporterFactory) (*Provider, error) {
	provider, sampler, err := newUnstartedProvider(opts)
	if err != nil {
		return nil, err
	}
	if err := provider.start(ctx, opts.Config, sampler, createExporter); err != nil {
		return nil, err
	}
	return provider, nil
}

// newUnstartedProvider 构造未启动的 provider facade 和采样器。
//
// Fx adapter 使用该函数在依赖图构造阶段提供稳定 facade，但不会连接 exporter 或启动 batch
// processor。返回的 provider 在 start 前通过 dynamic tracer 安全 no-op。
func newUnstartedProvider(opts Options) (*Provider, sdktrace.Sampler, error) {
	serviceName := trimSpace(opts.ServiceName)
	if serviceName == "" {
		return nil, nil, errors.New("tracing service name is required")
	}
	environment := trimSpace(opts.Environment)
	if environment == "" {
		return nil, nil, errors.New("tracing deployment environment is required")
	}
	sampleRatio := opts.Config.SampleRatio
	if sampleRatio < 0 || sampleRatio > 1 {
		return nil, nil, errors.New("tracing sample ratio must be between 0 and 1")
	}
	res := newResource(serviceName, environment, trimSpace(opts.Version), trimSpace(opts.InstanceID))
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))
	if !opts.Config.Enabled {
		sampler = sdktrace.NeverSample()
	}
	return &Provider{
		resource: res,
		propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	}, sampler, nil
}

// OTelTracerProvider 返回可在 constructor 阶段安全注入的 OpenTelemetry provider 视图。
func (p *Provider) OTelTracerProvider() trace.TracerProvider {
	if p == nil {
		return noop.NewTracerProvider()
	}
	return dynamicTracerProvider{TracerProvider: noop.NewTracerProvider(), provider: p}
}

// TextMapPropagator 返回 W3C trace context 与 baggage 组合传播器。
func (p *Provider) TextMapPropagator() propagation.TextMapPropagator {
	if p == nil {
		return propagation.NewCompositeTextMapPropagator()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.propagator == nil {
		return propagation.NewCompositeTextMapPropagator()
	}
	return p.propagator
}

// Tracer 返回底层 provider 创建的 tracer。
func (p *Provider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return dynamicTracer{provider: p, name: name, options: append([]trace.TracerOption(nil), opts...)}
}

// realTracer 返回当前底层 SDK provider 创建的 tracer。
//
// provider 为 nil、尚未启动或已经 Shutdown 时返回 no-op tracer，保证 constructor-time
// instrumentation 保存的 tracer 在任意生命周期阶段都可安全使用。
func (p *Provider) realTracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if p == nil {
		return noop.NewTracerProvider().Tracer(name, opts...)
	}
	p.mu.RLock()
	tp := p.tracerProvider
	p.mu.RUnlock()
	if tp == nil {
		return noop.NewTracerProvider().Tracer(name, opts...)
	}
	return tp.Tracer(name, opts...)
}

// Shutdown 关闭底层 SDK provider。
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	tp := p.tracerProvider
	p.tracerProvider = nil
	p.mu.Unlock()
	if tp == nil {
		return nil
	}
	return tp.Shutdown(ctx)
}
