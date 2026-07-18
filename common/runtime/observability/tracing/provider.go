package tracing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/aegiscore/common/runtime/config"
)

const (
	defaultOTLPTimeout = 5 * time.Second

	attributeServiceName           = "service.name"
	attributeDeploymentEnvironment = "deployment.environment"
	attributeServiceVersion        = "service.version"
	attributeServiceInstanceID     = "service.instance.id"
)

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

func newProvider(ctx context.Context, opts Options, createExporter exporterFactory) (*Provider, error) {
	provider, sampler, err := newUnstartedProvider(opts)
	if err != nil {
		return nil, err
	}
	if err := provider.Start(ctx, opts.Config, sampler, createExporter); err != nil {
		return nil, err
	}
	return provider, nil
}

func newUnstartedProvider(opts Options) (*Provider, sdktrace.Sampler, error) {
	serviceName := strings.TrimSpace(opts.ServiceName)
	if serviceName == "" {
		return nil, nil, errors.New("tracing service name is required")
	}
	environment := strings.TrimSpace(opts.Environment)
	if environment == "" {
		return nil, nil, errors.New("tracing deployment environment is required")
	}
	sampleRatio := opts.Config.SampleRatio
	if sampleRatio < 0 || sampleRatio > 1 {
		return nil, nil, errors.New("tracing sample ratio must be between 0 and 1")
	}
	res := newResource(serviceName, environment, strings.TrimSpace(opts.Version), strings.TrimSpace(opts.InstanceID))
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

func newOTLPExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	endpoint := strings.TrimSpace(cfg.OTLPEndpoint)
	if endpoint == "" {
		return nil, errors.New("otlp tracing endpoint is required")
	}
	clientOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTimeout(defaultOTLPTimeout),
	}
	if cfg.Insecure {
		clientOptions = append(clientOptions, otlptracegrpc.WithInsecure())
	}
	traceExporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(clientOptions...))
	if err != nil {
		return nil, wrapOTLPExporterError(err)
	}
	return traceExporter, nil
}

func wrapOTLPExporterError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("create OTLP tracing exporter: %w", err)
}

type dynamicTracerProvider struct {
	trace.TracerProvider
	provider *Provider
}

// OTelTracerProvider 返回可在 constructor 阶段安全注入的 OpenTelemetry provider 视图。
func (p *Provider) OTelTracerProvider() trace.TracerProvider {
	if p == nil {
		return noop.NewTracerProvider()
	}
	return dynamicTracerProvider{TracerProvider: noop.NewTracerProvider(), provider: p}
}

func (p dynamicTracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return p.provider.Tracer(name, opts...)
}

// Start 使用 lifecycle context 初始化底层 SDK provider。
func (p *Provider) Start(ctx context.Context, cfg config.TracingConfig, sampler sdktrace.Sampler, createExporter exporterFactory) error {
	if p == nil {
		return errors.New("tracing provider is required")
	}
	if ctx == nil {
		return errors.New("tracing provider context is required")
	}
	p.mu.RLock()
	res := p.resource
	p.mu.RUnlock()
	if res == nil {
		return errors.New("tracing resource is required")
	}
	tp, err := newTracerProvider(ctx, cfg, res, sampler, createExporter)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.tracerProvider = tp
	p.mu.Unlock()
	return nil
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

func newResource(serviceName string, environment string, version string, instanceID string) *resource.Resource {
	attrs := []attribute.KeyValue{
		attribute.String(attributeServiceName, serviceName),
		attribute.String(attributeDeploymentEnvironment, environment),
	}
	if version != "" {
		attrs = append(attrs, attribute.String(attributeServiceVersion, version))
	}
	if instanceID != "" {
		attrs = append(attrs, attribute.String(attributeServiceInstanceID, instanceID))
	}
	return resource.NewWithAttributes("", attrs...)
}
