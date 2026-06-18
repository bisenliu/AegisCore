package metrics

import (
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewProviderDisabledHasNoSideEffects(t *testing.T) {
	provider := newTestProvider(t, false, true)

	if provider.Enabled() {
		t.Fatal("provider is enabled")
	}
	if provider.Registerer() != nil {
		t.Fatal("disabled provider has registerer")
	}
	if provider.Gatherer() != nil {
		t.Fatal("disabled provider has gatherer")
	}

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_disabled_test_total",
		Help: "Disabled test counter.",
	})
	if err := provider.Register(counter); err != nil {
		t.Fatalf("Register on disabled provider: %v", err)
	}
	if err := provider.Register(nil); err != nil {
		t.Fatalf("Register nil on disabled provider: %v", err)
	}
}

func TestNewProviderEnabledCreatesRegistry(t *testing.T) {
	provider := newTestProvider(t, true, false)

	if !provider.Enabled() {
		t.Fatal("provider is disabled")
	}
	if provider.Registerer() == nil {
		t.Fatal("provider registerer is nil")
	}
	if provider.Gatherer() == nil {
		t.Fatal("provider gatherer is nil")
	}

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_enabled_test_total",
		Help: "Enabled test counter.",
	})
	if err := provider.Register(counter); err != nil {
		t.Fatalf("Register: %v", err)
	}
	counter.Inc()

	metric := firstMetric(t, gatherFamily(t, provider, "aegiscore_enabled_test_total"))
	assertMetricLabel(t, metric, LabelService, "aegiscore-test")
	assertMetricLabel(t, metric, LabelEnvironment, "local")
}

func TestRuntimeCollectorsRespectConfig(t *testing.T) {
	withRuntime := newTestProvider(t, true, true)
	assertHasFamily(t, withRuntime, "go_goroutines")
	assertHasFamily(t, withRuntime, "process_cpu_seconds_total")

	withoutRuntime := newTestProvider(t, true, false)
	assertMissingFamily(t, withoutRuntime, "go_goroutines")
	assertMissingFamily(t, withoutRuntime, "process_cpu_seconds_total")
}

func TestRegisterIgnoresAlreadyRegistered(t *testing.T) {
	provider := newTestProvider(t, true, false)
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_duplicate_test_total",
		Help: "Duplicate test counter.",
	})

	if err := provider.Register(counter); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := provider.Register(counter); err != nil {
		t.Fatalf("second Register: %v", err)
	}
}

func TestRegisterKeepsNonDuplicateErrors(t *testing.T) {
	provider := newTestProvider(t, true, false)
	first := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_conflict_test_total",
		Help: "First help.",
	})
	second := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_conflict_test_total",
		Help: "Second help.",
	})

	if err := provider.Register(first); err != nil {
		t.Fatalf("Register first: %v", err)
	}
	if err := provider.Register(second); err == nil {
		t.Fatal("Register second succeeded, want conflict error")
	} else if !strings.Contains(err.Error(), "register metrics collector") {
		t.Fatalf("error = %v, want wrapped register error", err)
	}
}

func TestRegisterRejectsNilCollector(t *testing.T) {
	provider := newTestProvider(t, true, false)

	if err := provider.Register(nil); !errors.Is(err, ErrNilCollector) {
		t.Fatalf("Register nil = %v, want ErrNilCollector", err)
	}
}

func TestMustRegisterDoesNotPanicOnDuplicate(t *testing.T) {
	provider := newTestProvider(t, true, false)
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_must_duplicate_test_total",
		Help: "Must duplicate test counter.",
	})

	provider.MustRegister(counter)
	provider.MustRegister(counter)
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
				Config:      testMetricsConfig(true, false),
				Environment: "local",
			},
			want: "service name",
		},
		{
			name: "environment",
			opts: Options{
				Config:      testMetricsConfig(true, false),
				ServiceName: "aegiscore-test",
			},
			want: "deployment environment",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider(tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestNewFxProviderUsesSharedConfig(t *testing.T) {
	provider, err := NewFxProvider(FxParams{
		Config: &config.Config{
			App: config.AppConfig{
				Name:        "aegiscore-test",
				Environment: "local",
			},
			Observability: config.ObservabilityConfig{
				Metrics: testMetricsConfig(true, false),
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFxProvider: %v", err)
	}
	if provider == nil || !provider.Enabled() {
		t.Fatalf("provider = %#v, want enabled provider", provider)
	}
}

func TestNewFxProviderRejectsMissingConfig(t *testing.T) {
	_, err := NewFxProvider(FxParams{})
	if err == nil || !strings.Contains(err.Error(), "metrics config") {
		t.Fatalf("error = %v, want metrics config error", err)
	}
}

func newTestProvider(t *testing.T, enabled bool, includeRuntime bool) *Provider {
	t.Helper()
	provider, err := NewProvider(Options{
		Config:      testMetricsConfig(enabled, includeRuntime),
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return provider
}

func testMetricsConfig(enabled bool, includeRuntime bool) config.MetricsConfig {
	return config.MetricsConfig{
		Enabled:        enabled,
		Path:           "/metrics",
		IncludeRuntime: includeRuntime,
	}
}

func gatherFamily(t *testing.T, provider *Provider, name string) *dto.MetricFamily {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("metric family %q not found", name)
	return nil
}

func firstMetric(t *testing.T, family *dto.MetricFamily) *dto.Metric {
	t.Helper()
	if len(family.GetMetric()) == 0 {
		t.Fatalf("metric family %q has no metrics", family.GetName())
	}
	return family.GetMetric()[0]
}

func assertHasFamily(t *testing.T, provider *Provider, name string) {
	t.Helper()
	_ = gatherFamily(t, provider, name)
}

func assertMissingFamily(t *testing.T, provider *Provider, name string) {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name {
			t.Fatalf("metric family %q exists, want missing", name)
		}
	}
}

func assertMetricLabel(t *testing.T, metric *dto.Metric, name string, want string) {
	t.Helper()
	for _, label := range metric.GetLabel() {
		if label.GetName() == name {
			if label.GetValue() != want {
				t.Fatalf("label %s = %q, want %q", name, label.GetValue(), want)
			}
			return
		}
	}
	t.Fatalf("label %s not found", name)
}
