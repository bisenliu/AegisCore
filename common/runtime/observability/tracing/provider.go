package tracing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aegiscore/common/runtime/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	exporterNone = "none"
	exporterOTLP = "otlp"

	attributeServiceName           = "service.name"
	attributeDeploymentEnvironment = "deployment.environment"
	attributeServiceVersion        = "service.version"
	attributeServiceInstanceID     = "service.instance.id"
)

var (
	// ErrOTLPExporterUnsupported 表示当前阶段尚未实现 OTLP exporter。
	ErrOTLPExporterUnsupported = errors.New("otlp tracing exporter is not implemented")
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
	if exporter == exporterOTLP {
		return nil, ErrOTLPExporterUnsupported
	}
	if exporter != exporterNone {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedExporter, exporter)
	}

	res := newResource(serviceName, environment, strings.TrimSpace(opts.Version), strings.TrimSpace(opts.InstanceID))
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))
	if !opts.Config.Enabled {
		sampler = sdktrace.NeverSample()
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	return &Provider{
		tracerProvider: tp,
		resource:       res,
		propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	}, nil
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
