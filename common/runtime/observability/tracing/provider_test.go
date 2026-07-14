package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
)

func TestEnabledProviderCreatesSampledSpanContext(t *testing.T) {
	provider := newTestProvider(t, 1.0)
	defer shutdownProvider(t, provider)

	_, span := provider.Tracer("test").Start(context.Background(), "operation")
	defer span.End()

	spanCtx := span.SpanContext()
	require.True(t, spanCtx.TraceID().IsValid(), "trace ID is invalid")
	require.True(t, spanCtx.SpanID().IsValid(), "span ID is invalid")
	require.True(t, spanCtx.IsSampled(), "span is not sampled")
}

func TestNewProviderWithZeroSampleRatioKeepsValidIDsButNotSampled(t *testing.T) {
	provider := newTestProvider(t, 0.0)
	defer shutdownProvider(t, provider)

	_, span := provider.Tracer("test").Start(context.Background(), "operation")
	defer span.End()

	spanCtx := span.SpanContext()
	require.True(t, spanCtx.TraceID().IsValid(), "trace ID is invalid")
	require.True(t, spanCtx.SpanID().IsValid(), "span ID is invalid")
	require.False(t, spanCtx.IsSampled(), "span is sampled")
}

func TestParentBasedSamplerHonorsSampledParent(t *testing.T) {
	provider := newTestProvider(t, 0.0)
	defer shutdownProvider(t, provider)
	parent := sampledRemoteParent(t)

	_, span := provider.Tracer("test").Start(parent, "child")
	defer span.End()

	require.True(t, span.SpanContext().IsSampled(), "child span is not sampled")
}

func TestProviderResourceAttributes(t *testing.T) {
	provider, err := newProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:      true,
			SampleRatio:  1.0,
			OTLPEndpoint: "collector.internal:4317",
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
		Version:     "1.2.3",
		InstanceID:  "instance-1",
	}, testExporterFactory)
	require.NoError(t, err, "NewProvider")
	defer shutdownProvider(t, provider)

	attrs := resourceAttributes(provider)
	assertAttribute(t, attrs, attributeServiceName, "aegiscore-test")
	assertAttribute(t, attrs, attributeDeploymentEnvironment, "local")
	assertAttribute(t, attrs, attributeServiceVersion, "1.2.3")
	assertAttribute(t, attrs, attributeServiceInstanceID, "instance-1")
}

func TestProviderShutdown(t *testing.T) {
	provider := newTestProvider(t, 1.0)

	require.NoError(t, provider.Shutdown(context.Background()), "Shutdown")
	require.NoError(t, provider.Shutdown(context.Background()), "second Shutdown")
}

func TestProviderTracerFallsBackToNoopWhenProviderIsNil(t *testing.T) {
	var provider *Provider

	tracer := provider.Tracer("test")
	require.NotNil(t, tracer, "Tracer = nil")
	_, span := tracer.Start(context.Background(), "operation")
	defer span.End()

	require.False(t, span.SpanContext().IsValid(), "span context is valid, want noop span")
}

func TestProviderTracerFallsBackToNoopWhenTracerProviderIsNil(t *testing.T) {
	provider := &Provider{}

	tracer := provider.Tracer("test")
	require.NotNil(t, tracer, "Tracer = nil")
	_, span := tracer.Start(context.Background(), "operation")
	defer span.End()

	require.False(t, span.SpanContext().IsValid(), "span context is valid, want noop span")
}

func TestNewProviderWithOTLPConfigCreatesProvider(t *testing.T) {
	provider, err := NewProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:      true,
			SampleRatio:  0.0,
			OTLPEndpoint: "collector.internal:4317",
			Insecure:     true,
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	require.NoError(t, err, "NewProvider")
	defer shutdownProvider(t, provider)

	_, span := provider.Tracer("test").Start(context.Background(), "operation")
	defer span.End()

	require.True(t, span.SpanContext().TraceID().IsValid(), "trace ID is invalid")
	require.False(t, span.SpanContext().IsSampled(), "span is sampled")
}

func TestNewProviderRejectsMissingOTLPEndpoint(t *testing.T) {
	_, err := NewProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:      true,
			SampleRatio:  1.0,
			OTLPEndpoint: "   ",
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	require.ErrorContains(t, err, "endpoint")
	require.NotContains(t, err.Error(), "collector.internal")
	require.NotContains(t, err.Error(), "secret")
}

func TestDisabledProviderDoesNotCreateOTLPExporter(t *testing.T) {
	called := false
	provider, err := newProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:     false,
			SampleRatio: 1.0,
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
	}, func(context.Context, config.TracingConfig) (sdktrace.SpanExporter, error) {
		called = true
		return &testSpanExporter{}, nil
	})
	require.NoError(t, err)
	defer shutdownProvider(t, provider)
	require.False(t, called)
}

func TestNewProviderRejectsMissingServiceIdentity(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "service name",
			opts: Options{
				Config:      testTracingConfig(1.0),
				Environment: "local",
			},
			want: "service name",
		},
		{
			name: "environment",
			opts: Options{
				Config:      testTracingConfig(1.0),
				ServiceName: "aegiscore-test",
			},
			want: "deployment environment",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider(context.Background(), tt.opts)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestNewProviderRejectsInvalidSampleRatio(t *testing.T) {
	_, err := NewProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:     true,
			SampleRatio: 1.1,
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	require.ErrorContains(t, err, "sample ratio")
}

func TestTextMapPropagatorUsesTraceContextAndBaggage(t *testing.T) {
	provider := newTestProvider(t, 1.0)
	defer shutdownProvider(t, provider)

	fields := provider.TextMapPropagator().Fields()
	assertContains(t, fields, "traceparent")
	assertContains(t, fields, "tracestate")
	assertContains(t, fields, "baggage")
}

func TestNewFxProviderRegistersShutdown(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	provider, err := NewFxProvider(FxParams{
		Lifecycle: lifecycle,
		Config: &config.Config{
			App: config.AppConfig{
				Name:        "aegiscore-test",
				Environment: "local",
			},
			Observability: config.ObservabilityConfig{
				Tracing: config.TracingConfig{Enabled: false, SampleRatio: 1.0},
			},
		},
	})
	require.NoError(t, err, "NewFxProvider")
	require.NotNil(t, provider, "provider is nil")
	require.Len(t, lifecycle.hooks, 1, "registered hooks")
	require.NotNil(t, lifecycle.hooks[0].OnStop, "registered OnStop hook")
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()), "OnStop")
}

func newTestProvider(t *testing.T, sampleRatio float64) *Provider {
	t.Helper()
	provider, err := newProvider(context.Background(), Options{
		Config:      testTracingConfig(sampleRatio),
		ServiceName: "aegiscore-test",
		Environment: "local",
	}, testExporterFactory)
	require.NoError(t, err, "NewProvider")
	return provider
}

func testTracingConfig(sampleRatio float64) config.TracingConfig {
	return config.TracingConfig{
		Enabled:      true,
		SampleRatio:  sampleRatio,
		OTLPEndpoint: "collector.internal:4317",
	}
}

type testSpanExporter struct{}

func (*testSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	return nil
}

func (*testSpanExporter) Shutdown(context.Context) error {
	return nil
}

func testExporterFactory(context.Context, config.TracingConfig) (sdktrace.SpanExporter, error) {
	return &testSpanExporter{}, nil
}

func shutdownProvider(t *testing.T, provider *Provider) {
	t.Helper()
	require.NoError(t, provider.Shutdown(context.Background()), "Shutdown")
}

func sampledRemoteParent(t *testing.T) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	require.NoError(t, err, "TraceIDFromHex")
	spanID, err := trace.SpanIDFromHex("0011223344556677")
	require.NoError(t, err, "SpanIDFromHex")
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	return trace.ContextWithRemoteSpanContext(context.Background(), spanCtx)
}

func resourceAttributes(provider *Provider) map[string]string {
	attrs := make(map[string]string)
	for _, attr := range provider.Resource().Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsString()
	}
	return attrs
}

func assertAttribute(t *testing.T, attrs map[string]string, key string, want string) {
	t.Helper()
	require.Equalf(t, want, attrs[key], "%s", key)
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	require.Contains(t, values, want)
}

type lifecycleRecorder struct {
	hooks []fx.Hook
}

func (r *lifecycleRecorder) Append(hook fx.Hook) {
	r.hooks = append(r.hooks, hook)
}
