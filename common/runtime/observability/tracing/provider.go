package tracing

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	exporterNone = "none"
	exporterOTLP = "otlp"

	defaultOTLPTimeout = 5 * time.Second

	attributeServiceName           = "service.name"
	attributeDeploymentEnvironment = "deployment.environment"
	attributeServiceVersion        = "service.version"
	attributeServiceInstanceID     = "service.instance.id"
)

var (
	// ErrUnsupportedExporter 表示 tracing exporter 不在当前支持集合中。
	ErrUnsupportedExporter = errors.New("unsupported tracing exporter")
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
	tracerProvider *sdktrace.TracerProvider
	resource       *resource.Resource
	propagator     propagation.TextMapPropagator
}

// NewProvider 基于配置创建本进程 OpenTelemetry tracer provider。
func NewProvider(ctx context.Context, opts Options) (*Provider, error) {
	if ctx == nil {
		return nil, errors.New("tracing provider context is required")
	}
	serviceName := strings.TrimSpace(opts.ServiceName)
	if serviceName == "" {
		return nil, errors.New("tracing service name is required")
	}
	environment := strings.TrimSpace(opts.Environment)
	if environment == "" {
		return nil, errors.New("tracing deployment environment is required")
	}
	sampleRatio := opts.Config.SampleRatio
	if sampleRatio < 0 || sampleRatio > 1 {
		return nil, errors.New("tracing sample ratio must be between 0 and 1")
	}
	exporter := strings.ToLower(strings.TrimSpace(opts.Config.Exporter))
	if exporter == "" {
		return nil, errors.New("tracing exporter is required")
	}
	if exporter != exporterNone && exporter != exporterOTLP {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedExporter, exporter)
	}

	res := newResource(serviceName, environment, strings.TrimSpace(opts.Version), strings.TrimSpace(opts.InstanceID))
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))
	if !opts.Config.Enabled {
		sampler = sdktrace.NeverSample()
		exporter = exporterNone
	}
	tp, err := newTracerProvider(ctx, opts.Config, exporter, res, sampler)
	if err != nil {
		return nil, err
	}
	return &Provider{
		tracerProvider: tp,
		resource:       res,
		propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	}, nil
}

func newTracerProvider(
	ctx context.Context,
	cfg config.TracingConfig,
	exporter string,
	res *resource.Resource,
	sampler sdktrace.Sampler,
) (*sdktrace.TracerProvider, error) {
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}
	switch exporter {
	case exporterNone:
		return sdktrace.NewTracerProvider(options...), nil
	case exporterOTLP:
		traceExporter, err := newOTLPExporter(ctx, cfg)
		if err != nil {
			return nil, err
		}
		options = append(options, sdktrace.WithBatcher(traceExporter))
		return sdktrace.NewTracerProvider(options...), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedExporter, exporter)
	}
}

func newOTLPExporter(ctx context.Context, cfg config.TracingConfig) (*otlptrace.Exporter, error) {
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
		return nil, errors.New("create otlp tracing exporter")
	}
	return traceExporter, nil
}

// TracerProvider 返回底层 OpenTelemetry SDK provider。
func (p *Provider) TracerProvider() *sdktrace.TracerProvider {
	if p == nil {
		return nil
	}
	return p.tracerProvider
}

// Resource 返回构造 provider 时绑定的 OpenTelemetry resource。
func (p *Provider) Resource() *resource.Resource {
	if p == nil {
		return nil
	}
	return p.resource
}

// TextMapPropagator 返回 W3C trace context 与 baggage 组合传播器。
func (p *Provider) TextMapPropagator() propagation.TextMapPropagator {
	if p == nil {
		return propagation.NewCompositeTextMapPropagator()
	}
	return p.propagator
}

// Tracer 返回底层 provider 创建的 tracer。
func (p *Provider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if p == nil || p.tracerProvider == nil {
		return noop.NewTracerProvider().Tracer(name, opts...)
	}
	return p.tracerProvider.Tracer(name, opts...)
}

// Shutdown 关闭底层 SDK provider。
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.tracerProvider == nil {
		return nil
	}
	return p.tracerProvider.Shutdown(ctx)
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
