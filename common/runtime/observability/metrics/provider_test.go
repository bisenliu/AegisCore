package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewProviderDisabledHasNoSideEffects(t *testing.T) {
	provider := newTestProvider(t, false, true)

	require.False(t, provider.Enabled(), "provider is enabled")
	require.Nil(t, provider.Registerer(), "disabled provider has registerer")
	require.Nil(t, provider.Gatherer(), "disabled provider has gatherer")

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_disabled_test_total",
		Help: "Disabled test counter.",
	})
	require.NoError(t, provider.Register(counter), "Register on disabled provider")
	require.NoError(t, provider.Register(nil), "Register nil on disabled provider")
}

func TestNewProviderEnabledCreatesRegistry(t *testing.T) {
	provider := newTestProvider(t, true, false)

	require.True(t, provider.Enabled(), "provider is disabled")
	require.NotNil(t, provider.Registerer(), "provider registerer is nil")
	require.NotNil(t, provider.Gatherer(), "provider gatherer is nil")

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_enabled_test_total",
		Help: "Enabled test counter.",
	})
	require.NoError(t, provider.Register(counter), "Register")
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

	require.NoError(t, provider.Register(counter), "Register")
	require.NoError(t, provider.Register(counter), "second Register")
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

	require.NoError(t, provider.Register(first), "Register first")
	err := provider.Register(second)
	require.ErrorContains(t, err, "register metrics collector")
}

func TestRegisterRejectsNilCollector(t *testing.T) {
	provider := newTestProvider(t, true, false)

	require.ErrorIs(t, provider.Register(nil), ErrNilCollector)
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
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestNewMetricsProviderUsesSharedConfig(t *testing.T) {
	provider, err := NewMetricsProvider(&config.Config{
		App: config.AppConfig{
			Name:        "aegiscore-test",
			Environment: "local",
		},
		Observability: config.ObservabilityConfig{
			Metrics: testMetricsConfig(true, false),
		},
	})
	require.NoError(t, err, "NewMetricsProvider")
	require.NotNil(t, provider)
	require.True(t, provider.Enabled(), "provider = %#v, want enabled provider", provider)
}

func TestNewMetricsProviderReturnsDisabledProvider(t *testing.T) {
	provider, err := NewMetricsProvider(&config.Config{
		App: config.AppConfig{
			Name:        "aegiscore-test",
			Environment: "local",
		},
		Observability: config.ObservabilityConfig{
			Metrics: testMetricsConfig(false, false),
		},
	})
	require.NoError(t, err, "NewMetricsProvider")
	require.NotNil(t, provider)
	require.False(t, provider.Enabled(), "provider = %#v, want disabled provider", provider)
}

func TestNewMetricsProviderRejectsMissingConfig(t *testing.T) {
	_, err := NewMetricsProvider(nil)
	require.ErrorContains(t, err, "metrics config")
}

func newTestProvider(t *testing.T, enabled bool, includeRuntime bool) *Provider {
	t.Helper()
	provider, err := NewProvider(Options{
		Config:      testMetricsConfig(enabled, includeRuntime),
		ServiceName: "aegiscore-test",
		Environment: "local",
	})
	require.NoError(t, err, "NewProvider")
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
	require.NoError(t, err, "Gather")
	var found *dto.MetricFamily
	for _, family := range families {
		if family.GetName() == name {
			found = family
			break
		}
	}
	require.NotNilf(t, found, "metric family %q not found", name)
	return found
}

func firstMetric(t *testing.T, family *dto.MetricFamily) *dto.Metric {
	t.Helper()
	require.NotEmptyf(t, family.GetMetric(), "metric family %q has no metrics", family.GetName())
	return family.GetMetric()[0]
}

func assertHasFamily(t *testing.T, provider *Provider, name string) {
	t.Helper()
	_ = gatherFamily(t, provider, name)
}

func assertMissingFamily(t *testing.T, provider *Provider, name string) {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	require.NoError(t, err, "Gather")
	for _, family := range families {
		require.NotEqualf(t, name, family.GetName(), "metric family %q exists, want missing", name)
	}
}

func assertMetricLabel(t *testing.T, metric *dto.Metric, name string, want string) {
	t.Helper()
	var got *string
	for _, label := range metric.GetLabel() {
		if label.GetName() == name {
			value := label.GetValue()
			got = &value
			break
		}
	}
	require.NotNilf(t, got, "label %s not found", name)
	require.Equalf(t, want, *got, "label %s", name)
}
