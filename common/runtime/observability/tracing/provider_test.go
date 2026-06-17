package tracing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewProviderWithNoneExporterCreatesSpanContext(t *testing.T) {
	provider := newTestProvider(t, 1.0)
	defer shutdownProvider(t, provider)

	_, span := provider.Tracer("test").Start(context.Background(), "operation")
	defer span.End()

	spanCtx := span.SpanContext()
	if !spanCtx.TraceID().IsValid() {
		t.Fatal("trace ID is invalid")
	}
	if !spanCtx.SpanID().IsValid() {
		t.Fatal("span ID is invalid")
	}
	if !spanCtx.IsSampled() {
		t.Fatal("span is not sampled")
	}
}

func TestNewProviderWithZeroSampleRatioKeepsValidIDsButNotSampled(t *testing.T) {
	provider := newTestProvider(t, 0.0)
	defer shutdownProvider(t, provider)

	_, span := provider.Tracer("test").Start(context.Background(), "operation")
	defer span.End()

	spanCtx := span.SpanContext()
	if !spanCtx.TraceID().IsValid() {
		t.Fatal("trace ID is invalid")
	}
	if !spanCtx.SpanID().IsValid() {
		t.Fatal("span ID is invalid")
	}
	if spanCtx.IsSampled() {
		t.Fatal("span is sampled")
	}
}

func TestParentBasedSamplerHonorsSampledParent(t *testing.T) {
	provider := newTestProvider(t, 0.0)
	defer shutdownProvider(t, provider)
	parent := sampledRemoteParent(t)

	_, span := provider.Tracer("test").Start(parent, "child")
	defer span.End()

	if !span.SpanContext().IsSampled() {
		t.Fatal("child span is not sampled")
	}
}

func TestProviderResourceAttributes(t *testing.T) {
	provider, err := NewProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:     true,
			SampleRatio: 1.0,
			Exporter:    exporterNone,
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
		Version:     "1.2.3",
		InstanceID:  "instance-1",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	defer shutdownProvider(t, provider)

	attrs := resourceAttributes(provider)
	assertAttribute(t, attrs, attributeServiceName, "aegiscore-test")
	assertAttribute(t, attrs, attributeDeploymentEnvironment, "local")
	assertAttribute(t, attrs, attributeServiceVersion, "1.2.3")
	assertAttribute(t, attrs, attributeServiceInstanceID, "instance-1")
}

func TestProviderShutdown(t *testing.T) {
	provider := newTestProvider(t, 1.0)

	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
}

func TestNewProviderRejectsUnsupportedExporter(t *testing.T) {
	_, err := NewProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:      true,
			SampleRatio:  1.0,
			Exporter:     exporterOTLP,
			OTLPEndpoint: "collector.internal:4317?token=secret",
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	if !errors.Is(err, ErrOTLPExporterUnsupported) {
		t.Fatalf("error = %v, want ErrOTLPExporterUnsupported", err)
	}
	if strings.Contains(err.Error(), "collector.internal") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked endpoint: %v", err)
	}
}

func TestNewProviderRejectsUnknownExporter(t *testing.T) {
	_, err := NewProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:     true,
			SampleRatio: 1.0,
			Exporter:    "zipkin",
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	if !errors.Is(err, ErrUnsupportedExporter) {
		t.Fatalf("error = %v, want ErrUnsupportedExporter", err)
	}
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
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestNewProviderRejectsInvalidSampleRatio(t *testing.T) {
	_, err := NewProvider(context.Background(), Options{
		Config: config.TracingConfig{
			Enabled:     true,
			SampleRatio: 1.1,
			Exporter:    exporterNone,
		},
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	if err == nil || !strings.Contains(err.Error(), "sample ratio") {
		t.Fatalf("error = %v, want sample ratio error", err)
	}
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
				Tracing: testTracingConfig(1.0),
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFxProvider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStop == nil {
		t.Fatalf("registered hooks = %#v, want one OnStop hook", lifecycle.hooks)
	}
	if err := lifecycle.hooks[0].OnStop(context.Background()); err != nil {
		t.Fatalf("OnStop: %v", err)
	}
}

func newTestProvider(t *testing.T, sampleRatio float64) *Provider {
	t.Helper()
	provider, err := NewProvider(context.Background(), Options{
		Config:      testTracingConfig(sampleRatio),
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return provider
}

func testTracingConfig(sampleRatio float64) config.TracingConfig {
	return config.TracingConfig{
		Enabled:     true,
		SampleRatio: sampleRatio,
		Exporter:    exporterNone,
	}
}

func shutdownProvider(t *testing.T, provider *Provider) {
	t.Helper()
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func sampledRemoteParent(t *testing.T) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0011223344556677")
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
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
	if got := attrs[key]; got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %v", want, values)
}

type lifecycleRecorder struct {
	hooks []fx.Hook
}

func (r *lifecycleRecorder) Append(hook fx.Hook) {
	r.hooks = append(r.hooks, hook)
}
