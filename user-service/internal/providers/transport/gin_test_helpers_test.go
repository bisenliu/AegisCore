package transport

import (
	"context"
	"net"
	"sync"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
)

func ginTestConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{Name: "configured-user-service", Environment: "test"},
		Observability: config.ObservabilityConfig{
			Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, OTLPEndpoint: "127.0.0.1:4317", Insecure: true},
		},
	}
}

func newGinTestTracingProvider(t *testing.T, cfg *config.Config) *commontracing.Provider {
	t.Helper()
	provider, _ := newGinTestTracingProviderWithCollector(t, cfg)
	return provider
}

func newGinTestTracingProviderWithCollector(t *testing.T, cfg *config.Config) (*commontracing.Provider, *testOTLPTraceService) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	collector := grpc.NewServer()
	service := &testOTLPTraceService{}
	collectortrace.RegisterTraceServiceServer(collector, service)
	go func() { _ = collector.Serve(listener) }()
	cfg.Observability.Tracing.OTLPEndpoint = listener.Addr().String()
	cfg.Observability.Tracing.Insecure = true
	provider, err := commontracing.NewProvider(context.Background(), commontracing.Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
		collector.GracefulStop()
		_ = listener.Close()
	})
	return provider, service
}

type testOTLPTraceService struct {
	collectortrace.UnimplementedTraceServiceServer
	mu    sync.Mutex
	spans []*tracepb.Span
}

func (s *testOTLPTraceService) Export(_ context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, resourceSpans := range req.GetResourceSpans() {
		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			s.spans = append(s.spans, scopeSpans.GetSpans()...)
		}
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func (s *testOTLPTraceService) Spans() []*tracepb.Span {
	s.mu.Lock()
	defer s.mu.Unlock()
	spans := make([]*tracepb.Span, len(s.spans))
	copy(spans, s.spans)
	return spans
}

func newGinTestTracingProviderWithRecorder(t *testing.T, cfg *config.Config) (*commontracing.Provider, *testOTLPTraceService) {
	t.Helper()
	return newGinTestTracingProviderWithCollector(t, cfg)
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

func exportedSpans(t *testing.T, provider *commontracing.Provider, collector *testOTLPTraceService) []*tracepb.Span {
	t.Helper()
	require.NoError(t, provider.Shutdown(context.Background()))
	return collector.Spans()
}

func endedGinSpan(t *testing.T, provider *commontracing.Provider, collector *testOTLPTraceService) *tracepb.Span {
	t.Helper()
	spans := exportedSpans(t, provider, collector)
	require.Len(t, spans, 1)
	return spans[0]
}

func spanHasEvent(span *tracepb.Span, name string) bool {
	for _, event := range span.GetEvents() {
		if event.GetName() == name {
			return true
		}
	}
	return false
}
