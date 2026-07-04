package providers

import (
	"context"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

func ginTestConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{Name: "configured-user-service", Environment: "test"},
		Observability: config.ObservabilityConfig{
			Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, Exporter: "none"},
		},
	}
}

func newGinTestTracingProvider(t *testing.T, cfg *config.Config) *commontracing.Provider {
	t.Helper()
	provider, err := commontracing.NewProvider(context.Background(), commontracing.Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return provider
}

func newGinTestTracingProviderWithRecorder(t *testing.T, cfg *config.Config) (*commontracing.Provider, *tracetest.SpanRecorder) {
	t.Helper()
	provider := newGinTestTracingProvider(t, cfg)
	recorder := tracetest.NewSpanRecorder()
	provider.TracerProvider().RegisterSpanProcessor(recorder)
	return provider, recorder
}

func newGinTestMetricsProvider(t *testing.T, cfg *config.Config) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      cfg.Observability.Metrics,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	require.NoError(t, err)
	return provider
}

func gatherGinMetricFamily(t *testing.T, provider *commonmetrics.Provider, name string) *dto.MetricFamily {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	require.NoError(t, err)
	var found *dto.MetricFamily
	for _, family := range families {
		if family.GetName() == name {
			found = family
			break
		}
	}
	require.NotNil(t, found, "metric family %q not found", name)
	return found
}

func findGinMetricByLabels(t *testing.T, family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	t.Helper()
	var found *dto.Metric
	for _, metric := range family.GetMetric() {
		if ginMetricHasLabels(metric, labels) {
			found = metric
			break
		}
	}
	require.NotNil(t, found, "metric family %q missing labels %#v", family.GetName(), labels)
	return found
}

func ginMetricHasLabels(metric *dto.Metric, labels map[string]string) bool {
	for key, want := range labels {
		found := false
		for _, label := range metric.GetLabel() {
			if label.GetName() == key {
				if label.GetValue() != want {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func assertGinMetricFamilyMissing(t *testing.T, provider *commonmetrics.Provider, name string) {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	require.NoError(t, err)
	for _, family := range families {
		require.NotEqual(t, name, family.GetName())
	}
}

func assertGinMetricFamilyMissingLabelValue(t *testing.T, family *dto.MetricFamily, value string) {
	t.Helper()
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			require.NotEqual(t, value, label.GetValue(), "metric family %q has forbidden label value", family.GetName())
		}
	}
}

func endedGinSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	return spans[0]
}

func spanHasEvent(span sdktrace.ReadOnlySpan, name string) bool {
	for _, event := range span.Events() {
		if event.Name == name {
			return true
		}
	}
	return false
}
